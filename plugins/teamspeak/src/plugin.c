/* ClipHub TeamSpeak 3 plugin: TS3 exports and the control worker.
 * One worker thread owns the state machine and every TS3 call that starts or stops a
 * recording. Voice callbacks only touch the recorder, which has its own lock. */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <windows.h>

#include "core.h"
#include "poll.h"
#include "recorder.h"
#include "ts3_functions.h"

#define EXPORT __declspec(dllexport)
#define PLUGIN_API_VERSION 26
#define LOST_AFTER_MS 120000
#define PATH_BUF 1024

enum { MENU_ENABLE = 1, MENU_DISABLE = 2, MENU_TOGGLE = 3 };
enum { EV_ENABLE, EV_DISABLE, EV_HOTKEY, EV_DISCONNECT, EV_MENUS };

typedef struct {
    int type;
    uint64 schid;
} QueuedEvent;

static struct TS3Functions ts3;
static char g_plugin_id[128];
static wchar_t g_config_ini[PATH_BUF];

static HANDLE g_worker;
static HANDLE g_wake;
static volatile LONG g_quit;
static volatile LONG g_enabled;
static volatile LONG g_inited;

static CRITICAL_SECTION g_q;
static QueuedEvent g_queue[32];
static int g_qlen;
static int g_poll_pending;
static int g_poll_ok;
static CoreLive g_poll_live;

/* ---- Paths and config ---- */

static int profile_dir(wchar_t *out, size_t cap) {
    DWORD n = GetEnvironmentVariableW(L"CLIPHUB_STUDIO_USER_DATA", out, (DWORD)cap);
    if (n == 0 || n >= cap) {
        wchar_t appdata[PATH_BUF];
        n = GetEnvironmentVariableW(L"APPDATA", appdata, PATH_BUF);
        if (n == 0 || n >= PATH_BUF) return 0;
        _snwprintf(out, cap, L"%ls\\cliphub-studio", appdata);
        out[cap - 1] = 0;
    }
    n = (DWORD)wcslen(out);
    while (n > 3 && (out[n - 1] == L'\\' || out[n - 1] == L'/')) out[--n] = 0;
    return 1;
}

static int plugin_path(wchar_t *out, size_t cap, const wchar_t *suffix) {
    wchar_t prof[PATH_BUF];
    if (!profile_dir(prof, PATH_BUF)) return 0;
    _snwprintf(out, cap, L"%ls\\data\\voice-plugin%ls%ls", prof, suffix[0] ? L"\\" : L"", suffix);
    out[cap - 1] = 0;
    return 1;
}

static int load_enabled(void) {
    char text[256], v[16];
    size_t n;
    FILE *f = _wfopen(g_config_ini, L"rb");
    if (!f) return 0;
    n = fread(text, 1, sizeof(text) - 1, f);
    fclose(f);
    text[n] = 0;
    return core_ini_get(text, "enabled", v, sizeof(v)) && strcmp(v, "1") == 0;
}

static void save_enabled(int on) {
    wchar_t dir[PATH_BUF];
    wchar_t *slash;
    FILE *f;
    wcscpy(dir, g_config_ini);
    slash = wcsrchr(dir, L'\\');
    if (slash) {
        *slash = 0;
        rec_mkdir_p(dir);
    }
    f = _wfopen(g_config_ini, L"wb");
    if (!f) return;
    fputs(on ? "enabled=1\n" : "enabled=0\n", f);
    fclose(f);
}

/* ---- Event queue ---- */

static void post_event(int type, uint64 schid) {
    EnterCriticalSection(&g_q);
    if (g_qlen < (int)(sizeof(g_queue) / sizeof(g_queue[0]))) {
        g_queue[g_qlen].type = type;
        g_queue[g_qlen].schid = schid;
        g_qlen++;
    }
    LeaveCriticalSection(&g_q);
    SetEvent(g_wake);
}

static void poll_sink(int ok, const CoreLive *live) {
    EnterCriticalSection(&g_q);
    g_poll_ok = ok;
    if (ok) g_poll_live = *live;
    g_poll_pending = 1;
    LeaveCriticalSection(&g_q);
    SetEvent(g_wake);
}

/* ---- Worker: recording control ---- */

static CoreSm g_sm;
static CoreLive g_last_live;
static ULONGLONG g_last_ok_tick;
static int g_lost_sent;

/* The UI thread may be blocked in shutdown, waiting for this worker. */
static void say(const char *msg) {
    if (!g_quit) ts3.printMessageToCurrentTab(msg);
}

static void refresh_menus(void) {
    int on = (int)g_enabled;
    if (!g_plugin_id[0]) return;
    ts3.setPluginMenuEnabled(g_plugin_id, MENU_ENABLE, !on);
    ts3.setPluginMenuEnabled(g_plugin_id, MENU_DISABLE, on);
    ts3.setPluginMenuEnabled(g_plugin_id, MENU_TOGGLE, on);
}

static int do_start(CoreTrigger trigger) {
    uint64 schid = ts3.getCurrentServerConnectionHandlerID();
    int status = 0, ok;
    anyID me = 0;
    uint64 chan_id = 0;
    char *server = NULL, *chan = NULL;
    wchar_t inbox[PATH_BUF];
    RecStartOpts o;

    if (ts3.getConnectionStatus(schid, &status) != 0 || status != STATUS_CONNECTION_ESTABLISHED) {
        if (trigger == CORE_TRIG_MANUAL) say("[ClipHub] Conéctate a un servidor de TeamSpeak para grabar.");
        return 0;
    }
    if (!plugin_path(inbox, PATH_BUF, L"inbox")) return 0;
    if (ts3.getClientID(schid, &me) == 0 && ts3.getChannelOfClient(schid, me, &chan_id) == 0)
        ts3.getChannelVariableAsString(schid, chan_id, CHANNEL_NAME, &chan);
    ts3.getServerVariableAsString(schid, VIRTUALSERVER_NAME, &server);

    o.inbox = inbox;
    o.trigger = trigger;
    o.server = server;
    o.channel = chan;
    o.schid = schid;
    ok = rec_start(&o);
    if (server) ts3.freeMemory(server);
    if (chan) ts3.freeMemory(chan);
    if (!ok) {
        if (trigger == CORE_TRIG_MANUAL) say("[ClipHub] No se pudo crear la carpeta de grabación.");
        return 0;
    }
    ts3.startVoiceRecording(schid);
    say("[ClipHub] Grabando la voz del equipo para ClipHub Studio.");
    if (g_last_live.live) rec_update_live(&g_last_live);
    return 1;
}

static void do_stop(void) {
    uint64 schid = 0;
    int n;
    char msg[160];
    if (!rec_active(&schid)) return;
    n = rec_stop();
    if (n < 0) return;
    ts3.stopVoiceRecording(schid);
    snprintf(msg, sizeof(msg), "[ClipHub] Grabación guardada (%d %s). Studio la asociará a la partida.", n, n == 1 ? "pista" : "pistas");
    say(msg);
}

static void apply(CoreEvent ev, const char *session) {
    CoreResult r = core_sm_step(&g_sm, ev, session);
    switch (r.action) {
    case CORE_ACT_START:
        if (!do_start(r.trigger)) core_sm_recording_ended(&g_sm);
        break;
    case CORE_ACT_STOP:
        do_stop();
        break;
    case CORE_ACT_RESTART:
        do_stop();
        if (!do_start(r.trigger)) core_sm_recording_ended(&g_sm);
        break;
    case CORE_ACT_NONE:
        break;
    }
}

static void enable_recording(int persist) {
    wchar_t studio_ini[PATH_BUF], inbox[PATH_BUF];
    if (g_enabled) return;
    if (persist) save_enabled(1);
    g_enabled = 1;
    apply(CORE_EV_ENABLE, "");
    g_last_ok_tick = GetTickCount64();
    g_lost_sent = 0;
    memset(&g_last_live, 0, sizeof(g_last_live));
    if (plugin_path(inbox, PATH_BUF, L"inbox")) rec_recover(inbox);
    if (plugin_path(studio_ini, PATH_BUF, L"studio.ini")) poll_start(studio_ini, poll_sink);
    refresh_menus();
}

static void disable_recording(void) {
    if (!g_enabled) return;
    g_enabled = 0;
    poll_stop();
    EnterCriticalSection(&g_q);
    g_poll_pending = 0;
    LeaveCriticalSection(&g_q);
    apply(CORE_EV_DISABLE, "");
    save_enabled(0);
    refresh_menus();
}

static void handle_event(const QueuedEvent *e) {
    switch (e->type) {
    case EV_ENABLE:
        enable_recording(1);
        break;
    case EV_DISABLE:
        disable_recording();
        break;
    case EV_HOTKEY:
        if (!g_enabled) say("[ClipHub] La grabación está desactivada. Actívala en el menú Plugins > ClipHub.");
        else apply(CORE_EV_HOTKEY, "");
        break;
    case EV_DISCONNECT: {
        uint64 s = 0;
        if (rec_active(&s) && s == e->schid) {
            do_stop();
            core_sm_recording_ended(&g_sm);
        }
        break;
    }
    case EV_MENUS:
        refresh_menus();
        break;
    }
}

static void handle_poll(void) {
    int ok, pending;
    CoreLive *live = (CoreLive *)malloc(sizeof(*live));
    if (!live) return;
    EnterCriticalSection(&g_q);
    pending = g_poll_pending;
    ok = g_poll_ok;
    if (pending && ok) *live = g_poll_live;
    g_poll_pending = 0;
    LeaveCriticalSection(&g_q);
    if (pending && g_enabled && ok) {
        g_last_ok_tick = GetTickCount64();
        g_lost_sent = 0;
        g_last_live = *live;
        if (live->live) {
            apply(CORE_EV_LIVE_START, live->session);
            rec_update_live(live);
        } else {
            apply(CORE_EV_LIVE_END, "");
        }
    }
    free(live);
}

static DWORD WINAPI worker_main(LPVOID arg) {
    (void)arg;
    if (load_enabled()) enable_recording(0);
    for (;;) {
        QueuedEvent ev;
        int have;
        WaitForSingleObject(g_wake, g_enabled ? 1000 : INFINITE);
        if (g_quit) break;
        for (;;) {
            EnterCriticalSection(&g_q);
            have = g_qlen > 0;
            if (have) {
                ev = g_queue[0];
                memmove(&g_queue[0], &g_queue[1], sizeof(g_queue[0]) * (size_t)(g_qlen - 1));
                g_qlen--;
            }
            LeaveCriticalSection(&g_q);
            if (!have) break;
            handle_event(&ev);
        }
        if (g_enabled) {
            ULONGLONG now = GetTickCount64();
            handle_poll();
            if (!g_lost_sent && now - g_last_ok_tick > LOST_AFTER_MS) {
                g_lost_sent = 1;
                apply(CORE_EV_STUDIO_LOST, "");
            }
            rec_tick(now);
        }
    }
    poll_stop();
    do_stop();
    return 0;
}

/* ---- Voice callbacks ---- */

static void fetch_identity(uint64 schid, anyID client_id, char *uid, size_t uidcap, char *nick, size_t nickcap) {
    char *u = NULL, *n = NULL;
    if (ts3.getClientVariableAsString(schid, client_id, CLIENT_UNIQUE_IDENTIFIER, &u) == 0 && u && u[0]) snprintf(uid, uidcap, "%s", u);
    /* TeamSpeak reuses client ids, so without an identity the nickname names the speaker. */
    else uid[0] = 0;
    if (ts3.getClientVariableAsString(schid, client_id, CLIENT_NICKNAME, &n) == 0 && n) snprintf(nick, nickcap, "%s", n);
    else nick[0] = 0;
    if (u) ts3.freeMemory(u);
    if (n) ts3.freeMemory(n);
}

EXPORT void ts3plugin_onEditPlaybackVoiceDataEvent(uint64 schid, anyID clientID, short *samples, int sampleCount, int channels) {
    RecHandle h;
    uint64 rs = 0;
    if (!g_inited || !rec_active(&rs) || rs != schid) return;
    if (!rec_find_track(schid, clientID, &h)) {
        char uid[CORE_STR_MAX], nick[CORE_STR_MAX];
        RecTrackInfo info;
        fetch_identity(schid, clientID, uid, sizeof(uid), nick, sizeof(nick));
        info.client_id = clientID;
        info.uid = uid;
        info.nickname = nick;
        info.self = 0;
        if (!rec_add_track(schid, &info, &h)) return;
    }
    rec_write(&h, samples, sampleCount, channels);
}

EXPORT void ts3plugin_onEditCapturedVoiceDataEvent(uint64 schid, short *samples, int sampleCount, int channels, int *edited) {
    RecHandle h;
    uint64 rs = 0;
    if (!g_inited || !edited || !(*edited & 2) || !rec_active(&rs) || rs != schid) return;
    if (!rec_find_self(schid, &h)) {
        char uid[CORE_STR_MAX], nick[CORE_STR_MAX];
        RecTrackInfo info;
        anyID me = 0;
        if (ts3.getClientID(schid, &me) != 0) return;
        fetch_identity(schid, me, uid, sizeof(uid), nick, sizeof(nick));
        info.client_id = me;
        info.uid = uid;
        info.nickname = nick;
        info.self = 1;
        if (!rec_add_track(schid, &info, &h)) return;
    }
    rec_write(&h, samples, sampleCount, channels);
}

EXPORT void ts3plugin_onConnectStatusChangeEvent(uint64 schid, int newStatus, unsigned int errorNumber) {
    (void)errorNumber;
    if (g_inited && newStatus == STATUS_DISCONNECTED) post_event(EV_DISCONNECT, schid);
}

/* ---- Required exports ---- */

EXPORT const char *ts3plugin_name(void) { return "ClipHub"; }
EXPORT const char *ts3plugin_version(void) { return CORE_PLUGIN_VERSION; }
EXPORT int ts3plugin_apiVersion(void) { return PLUGIN_API_VERSION; }
EXPORT const char *ts3plugin_author(void) { return "ClipHub"; }
EXPORT const char *ts3plugin_description(void) { return "Graba la voz del equipo, una pista por persona, y la entrega a ClipHub Studio para el replay."; }
EXPORT void ts3plugin_setFunctionPointers(const struct TS3Functions funcs) { ts3 = funcs; }

EXPORT int ts3plugin_init(void) {
    char cfg[PATH_BUF];
    wchar_t wcfg[PATH_BUF];
    size_t n;
    ts3.getConfigPath(cfg, sizeof(cfg));
    if (!rec_utf8_to_wide(cfg, wcfg, PATH_BUF)) return 1;
    n = wcslen(wcfg);
    while (n > 0 && (wcfg[n - 1] == L'\\' || wcfg[n - 1] == L'/')) wcfg[--n] = 0;
    _snwprintf(g_config_ini, PATH_BUF, L"%ls\\plugins\\cliphub_teamspeak.ini", wcfg);
    g_config_ini[PATH_BUF - 1] = 0;

    InitializeCriticalSection(&g_q);
    g_qlen = 0;
    g_poll_pending = 0;
    g_quit = 0;
    g_enabled = 0;
    core_sm_init(&g_sm);
    rec_init();
    g_wake = CreateEventW(NULL, FALSE, FALSE, NULL);
    g_worker = g_wake ? CreateThread(NULL, 0, worker_main, NULL, 0, NULL) : NULL;
    if (!g_worker) {
        rec_shutdown();
        if (g_wake) CloseHandle(g_wake);
        DeleteCriticalSection(&g_q);
        return 1;
    }
    g_inited = 1;
    return 0;
}

EXPORT void ts3plugin_shutdown(void) {
    if (!g_inited) return;
    g_inited = 0;
    g_quit = 1;
    SetEvent(g_wake);
    WaitForSingleObject(g_worker, INFINITE);
    CloseHandle(g_worker);
    CloseHandle(g_wake);
    g_worker = NULL;
    g_wake = NULL;
    rec_shutdown();
    DeleteCriticalSection(&g_q);
}

/* ---- Optional exports ---- */

EXPORT void ts3plugin_registerPluginID(const char *id) { snprintf(g_plugin_id, sizeof(g_plugin_id), "%s", id ? id : ""); }
EXPORT const char *ts3plugin_infoTitle(void) { return "ClipHub"; }
EXPORT void ts3plugin_freeMemory(void *data) { free(data); }
EXPORT int ts3plugin_requestAutoload(void) { return 0; }

EXPORT void ts3plugin_infoData(uint64 schid, uint64 id, enum PluginItemType type, char **data) {
    const char *text;
    uint64 rs = 0;
    (void)id;
    *data = NULL;
    if (type != PLUGIN_SERVER) return;
    if (!g_enabled) text = "ClipHub: grabación desactivada";
    else if (rec_active(&rs) && rs == schid) text = "ClipHub: grabando";
    else text = "ClipHub: grabación activada, esperando partida";
    *data = (char *)malloc(strlen(text) + 1);
    if (*data) strcpy(*data, text);
}

static struct PluginMenuItem *make_item(int id, const char *text) {
    struct PluginMenuItem *m = (struct PluginMenuItem *)calloc(1, sizeof(*m));
    if (!m) return NULL;
    m->type = PLUGIN_MENU_TYPE_GLOBAL;
    m->id = id;
    snprintf(m->text, PLUGIN_MENU_BUFSZ, "%s", text);
    return m;
}

EXPORT void ts3plugin_initMenus(struct PluginMenuItem ***menuItems, char **menuIcon) {
    *menuItems = (struct PluginMenuItem **)calloc(4, sizeof(struct PluginMenuItem *));
    *menuIcon = NULL;
    if (!*menuItems) return;
    (*menuItems)[0] = make_item(MENU_ENABLE, "Activar la grabación de ClipHub");
    (*menuItems)[1] = make_item(MENU_DISABLE, "Desactivar la grabación de ClipHub");
    (*menuItems)[2] = make_item(MENU_TOGGLE, "Empezar o parar la grabación");
    (*menuItems)[3] = NULL;
    if (g_inited) post_event(EV_MENUS, 0);
}

EXPORT void ts3plugin_onMenuItemEvent(uint64 schid, enum PluginMenuType type, int menuItemID, uint64 selectedItemID) {
    (void)schid;
    (void)selectedItemID;
    if (!g_inited || type != PLUGIN_MENU_TYPE_GLOBAL) return;
    if (menuItemID == MENU_ENABLE) post_event(EV_ENABLE, 0);
    else if (menuItemID == MENU_DISABLE) post_event(EV_DISABLE, 0);
    else if (menuItemID == MENU_TOGGLE) post_event(EV_HOTKEY, 0);
}

EXPORT void ts3plugin_initHotkeys(struct PluginHotkey ***hotkeys) {
    *hotkeys = (struct PluginHotkey **)calloc(2, sizeof(struct PluginHotkey *));
    if (!*hotkeys) return;
    (*hotkeys)[0] = (struct PluginHotkey *)calloc(1, sizeof(struct PluginHotkey));
    if ((*hotkeys)[0]) {
        snprintf((*hotkeys)[0]->keyword, PLUGIN_HOTKEY_BUFSZ, "%s", "cliphub_toggle");
        snprintf((*hotkeys)[0]->description, PLUGIN_HOTKEY_BUFSZ, "%s", "Empezar o parar la grabación de ClipHub");
    }
    (*hotkeys)[1] = NULL;
}

EXPORT void ts3plugin_onHotkeyEvent(const char *keyword) {
    if (g_inited && keyword && strcmp(keyword, "cliphub_toggle") == 0) post_event(EV_HOTKEY, 0);
}
