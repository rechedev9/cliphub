#include "poll.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <windows.h>
#include <winhttp.h>

#ifndef WINHTTP_ACCESS_TYPE_NO_PROXY
#define WINHTTP_ACCESS_TYPE_NO_PROXY 1
#endif

#define POLL_INTERVAL_MS 2000
#define TIMEOUT_MS 2000
#define BODY_MAX (64 * 1024)

static HANDLE g_thread;
static HANDLE g_stop;
static wchar_t g_ini[1024];
static PollSink g_sink;

static int read_origin(char *host, size_t hostcap, int *port) {
    char text[8192];
    char origin[256];
    size_t n;
    FILE *f = _wfopen(g_ini, L"rb");
    if (!f) return 0;
    n = fread(text, 1, sizeof(text) - 1, f);
    fclose(f);
    text[n] = 0;
    if (!core_ini_get(text, "web_origin", origin, sizeof(origin))) return 0;
    return core_parse_origin(origin, host, hostcap, port);
}

/* Returns a malloc'd body on HTTP 200, NULL otherwise. */
static char *http_get_live(const char *host, int port) {
    wchar_t whost[64];
    HINTERNET ses = NULL, con = NULL, req = NULL;
    char *body = NULL;
    size_t len = 0;
    DWORD status = 0, size = sizeof(status);
    int i;

    for (i = 0; host[i] && i < 63; i++) whost[i] = (wchar_t)host[i];
    whost[i] = 0;

    ses = WinHttpOpen(L"ClipHubTeamSpeak/1.0", WINHTTP_ACCESS_TYPE_NO_PROXY, WINHTTP_NO_PROXY_NAME, WINHTTP_NO_PROXY_BYPASS, 0);
    if (!ses) goto done;
    WinHttpSetTimeouts(ses, TIMEOUT_MS, TIMEOUT_MS, TIMEOUT_MS, TIMEOUT_MS);
    con = WinHttpConnect(ses, whost, (INTERNET_PORT)port, 0);
    if (!con) goto done;
    req = WinHttpOpenRequest(con, L"GET", L"/api/voice-plugin/live", NULL, WINHTTP_NO_REFERER, WINHTTP_DEFAULT_ACCEPT_TYPES, 0);
    if (!req) goto done;
    if (!WinHttpSendRequest(req, WINHTTP_NO_ADDITIONAL_HEADERS, 0, WINHTTP_NO_REQUEST_DATA, 0, 0, 0)) goto done;
    if (!WinHttpReceiveResponse(req, NULL)) goto done;
    if (!WinHttpQueryHeaders(req, WINHTTP_QUERY_STATUS_CODE | WINHTTP_QUERY_FLAG_NUMBER, WINHTTP_HEADER_NAME_BY_INDEX, &status, &size, WINHTTP_NO_HEADER_INDEX)) goto done;
    if (status != 200) goto done;
    body = (char *)malloc(BODY_MAX + 1);
    if (!body) goto done;
    for (;;) {
        DWORD got = 0;
        if (len >= BODY_MAX) break;
        if (!WinHttpReadData(req, body + len, (DWORD)(BODY_MAX - len), &got)) {
            free(body);
            body = NULL;
            goto done;
        }
        if (got == 0) break;
        len += got;
    }
    body[len] = 0;
done:
    if (req) WinHttpCloseHandle(req);
    if (con) WinHttpCloseHandle(con);
    if (ses) WinHttpCloseHandle(ses);
    return body;
}

static void poll_once(void) {
    char host[32];
    int port = 0;
    char *body;
    CoreLive *live;
    if (!read_origin(host, sizeof(host), &port)) {
        g_sink(0, NULL);
        return;
    }
    body = http_get_live(host, port);
    if (!body) {
        g_sink(0, NULL);
        return;
    }
    live = (CoreLive *)malloc(sizeof(*live));
    if (!live) {
        free(body);
        g_sink(0, NULL);
        return;
    }
    core_live_parse(body, live);
    g_sink(1, live);
    free(live);
    free(body);
}

static DWORD WINAPI poll_main(LPVOID arg) {
    (void)arg;
    do {
        poll_once();
    } while (WaitForSingleObject(g_stop, POLL_INTERVAL_MS) == WAIT_TIMEOUT);
    return 0;
}

int poll_start(const wchar_t *studio_ini, PollSink sink) {
    if (g_thread) return 1;
    if (wcslen(studio_ini) >= sizeof(g_ini) / sizeof(g_ini[0])) return 0;
    wcscpy(g_ini, studio_ini);
    g_sink = sink;
    g_stop = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!g_stop) return 0;
    g_thread = CreateThread(NULL, 0, poll_main, NULL, 0, NULL);
    if (!g_thread) {
        CloseHandle(g_stop);
        g_stop = NULL;
        return 0;
    }
    return 1;
}

void poll_stop(void) {
    if (!g_thread) return;
    SetEvent(g_stop);
    WaitForSingleObject(g_thread, INFINITE);
    CloseHandle(g_thread);
    CloseHandle(g_stop);
    g_thread = NULL;
    g_stop = NULL;
}
