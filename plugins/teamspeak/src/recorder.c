#include "recorder.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <windows.h>

#define DIR_MAX 1024
#define MANIFEST_EVERY_MS 30000
#define RECOVER_MIN_AGE_MS 90000

typedef struct {
    int used;
    unsigned client_id;
    int self;
    char uid[CORE_STR_MAX];
    char nick[CORE_STR_MAX];
    char base[64];
    FILE *pcm;
    FILE *idx;
    CoreRun run;
} Track;

static struct {
    CRITICAL_SECTION lock;
    int active;
    unsigned gen;
    uint64_t schid;
    wchar_t dir[DIR_MAX];
    LARGE_INTEGER qpc0;
    LARGE_INTEGER freq;
    int64_t started_ms;
    CoreTrigger trigger;
    char server[CORE_STR_MAX];
    char channel[CORE_STR_MAX];
    char map[64];
    char steamid[24];
    char session[48];
    int score_ct;
    int score_t;
    CoreRound rounds[CORE_MAX_ROUNDS];
    int nrounds;
    Track tracks[CORE_MAX_TRACKS];
    int ntracks;
    int dirty;
    uint64_t last_manifest_ms;
} R;

void rec_init(void) {
    memset(&R, 0, sizeof(R));
    InitializeCriticalSection(&R.lock);
    QueryPerformanceFrequency(&R.freq);
}

void rec_shutdown(void) {
    rec_stop();
    DeleteCriticalSection(&R.lock);
}

int rec_utf8_to_wide(const char *s, wchar_t *out, int cap) { return MultiByteToWideChar(CP_UTF8, 0, s, -1, out, cap) > 0; }

int rec_wide_to_utf8(const wchar_t *s, char *out, int cap) { return WideCharToMultiByte(CP_UTF8, 0, s, -1, out, cap, NULL, NULL) > 0; }

void rec_mkdir_p(const wchar_t *path) {
    wchar_t tmp[DIR_MAX];
    size_t i, n = wcslen(path);
    if (n == 0 || n >= DIR_MAX) return;
    wcscpy(tmp, path);
    for (i = 3; i < n; i++) {
        if (tmp[i] == L'\\' || tmp[i] == L'/') {
            wchar_t c = tmp[i];
            tmp[i] = 0;
            CreateDirectoryW(tmp, NULL);
            tmp[i] = c;
        }
    }
    CreateDirectoryW(tmp, NULL);
}

static int64_t now_unix_ms(void) {
    FILETIME ft;
    ULARGE_INTEGER u;
    GetSystemTimePreciseAsFileTime(&ft);
    u.LowPart = ft.dwLowDateTime;
    u.HighPart = ft.dwHighDateTime;
    return core_filetime_to_unix_ms(u.QuadPart);
}

static int write_file_atomic(const wchar_t *dir, const char *text) {
    wchar_t tmp[DIR_MAX], fin[DIR_MAX];
    FILE *f;
    size_t n = strlen(text);
    int ok;
    _snwprintf(tmp, DIR_MAX, L"%ls\\manifest.json.tmp", dir);
    _snwprintf(fin, DIR_MAX, L"%ls\\manifest.json", dir);
    tmp[DIR_MAX - 1] = fin[DIR_MAX - 1] = 0;
    f = _wfopen(tmp, L"wb");
    if (!f) return 0;
    ok = fwrite(text, 1, n, f) == n;
    ok = fclose(f) == 0 && ok;
    if (!ok) return 0;
    return MoveFileExW(tmp, fin, MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH) != 0;
}

/* Caller holds the lock. */
static char *manifest_json_locked(int complete, int64_t ended_ms) {
    CoreManifest *m = (CoreManifest *)calloc(1, sizeof(*m));
    char *json;
    int i;
    if (!m) return NULL;
    m->complete = complete;
    m->started_at_ms = R.started_ms;
    m->ended_at_ms = complete ? ended_ms : 0;
    m->trigger = R.trigger;
    memcpy(m->server, R.server, sizeof(m->server));
    memcpy(m->channel, R.channel, sizeof(m->channel));
    memcpy(m->map, R.map, sizeof(m->map));
    memcpy(m->steamid, R.steamid, sizeof(m->steamid));
    memcpy(m->session, R.session, sizeof(m->session));
    m->score_ct = R.score_ct;
    m->score_t = R.score_t;
    m->nrounds = R.nrounds;
    memcpy(m->rounds, R.rounds, sizeof(CoreRound) * (size_t)R.nrounds);
    for (i = 0; i < R.ntracks; i++) {
        snprintf(m->tracks[i].file, sizeof(m->tracks[i].file), "%s", R.tracks[i].base);
        snprintf(m->tracks[i].nickname, sizeof(m->tracks[i].nickname), "%s", R.tracks[i].nick);
        snprintf(m->tracks[i].uid, sizeof(m->tracks[i].uid), "%s", R.tracks[i].uid);
        m->tracks[i].self = R.tracks[i].self;
    }
    m->ntracks = R.ntracks;
    json = core_manifest_json(m);
    free(m);
    return json;
}

static unsigned random16(void) {
    static unsigned counter;
    LARGE_INTEGER q;
    unsigned x;
    QueryPerformanceCounter(&q);
    x = (unsigned)q.QuadPart ^ (GetCurrentProcessId() * 2654435761u) ^ (++counter * 40503u);
    x ^= x >> 13;
    x *= 0x5bd1e995u;
    x ^= x >> 15;
    return x & 0xffffu;
}

int rec_start(const RecStartOpts *o) {
    wchar_t dir[DIR_MAX];
    wchar_t wname[64];
    char name[40];
    char *json;
    int tries, made = 0;
    LARGE_INTEGER q0;
    int64_t ms;

    EnterCriticalSection(&R.lock);
    if (R.active) {
        LeaveCriticalSection(&R.lock);
        return 0;
    }
    LeaveCriticalSection(&R.lock);

    rec_mkdir_p(o->inbox);
    ms = now_unix_ms();
    for (tries = 0; tries < 8 && !made; tries++) {
        core_folder_name(name, sizeof(name), ms, random16());
        rec_utf8_to_wide(name, wname, 64);
        _snwprintf(dir, DIR_MAX, L"%ls\\%ls", o->inbox, wname);
        dir[DIR_MAX - 1] = 0;
        made = CreateDirectoryW(dir, NULL) != 0;
    }
    if (!made) return 0;

    EnterCriticalSection(&R.lock);
    memset(R.tracks, 0, sizeof(R.tracks));
    R.ntracks = 0;
    wcscpy(R.dir, dir);
    QueryPerformanceCounter(&q0);
    R.qpc0 = q0;
    R.started_ms = now_unix_ms();
    R.trigger = o->trigger;
    snprintf(R.server, sizeof(R.server), "%s", o->server ? o->server : "");
    snprintf(R.channel, sizeof(R.channel), "%s", o->channel ? o->channel : "");
    R.map[0] = R.steamid[0] = R.session[0] = 0;
    R.score_ct = R.score_t = -1;
    R.nrounds = 0;
    R.schid = o->schid;
    R.dirty = 0;
    R.last_manifest_ms = (uint64_t)GetTickCount64();
    json = manifest_json_locked(0, 0);
    LeaveCriticalSection(&R.lock);

    if (!json || !write_file_atomic(dir, json)) {
        free(json);
        RemoveDirectoryW(dir);
        return 0;
    }
    free(json);
    EnterCriticalSection(&R.lock);
    R.active = 1;
    LeaveCriticalSection(&R.lock);
    return 1;
}

static void close_track_locked(Track *t) {
    int64_t s, c;
    char line[64];
    if (core_run_close(&t->run, &s, &c) && t->idx) {
        core_idx_line(line, sizeof(line), s, c);
        fputs(line, t->idx);
    }
    if (t->pcm) fclose(t->pcm);
    if (t->idx) fclose(t->idx);
    t->pcm = t->idx = NULL;
}

int rec_stop(void) {
    char *json;
    wchar_t dir[DIR_MAX];
    int i, n;
    EnterCriticalSection(&R.lock);
    if (!R.active) {
        LeaveCriticalSection(&R.lock);
        return -1;
    }
    R.active = 0;
    R.gen++;
    for (i = 0; i < R.ntracks; i++) close_track_locked(&R.tracks[i]);
    n = R.ntracks;
    wcscpy(dir, R.dir);
    json = manifest_json_locked(1, now_unix_ms());
    R.ntracks = 0;
    LeaveCriticalSection(&R.lock);
    if (json) {
        write_file_atomic(dir, json);
        free(json);
    }
    return n;
}

int rec_active(uint64_t *schid) {
    int a;
    EnterCriticalSection(&R.lock);
    a = R.active;
    if (a && schid) *schid = R.schid;
    LeaveCriticalSection(&R.lock);
    return a;
}

int rec_find_track(uint64_t schid, unsigned client_id, RecHandle *h) {
    int i, found = 0;
    EnterCriticalSection(&R.lock);
    if (R.active && R.schid == schid) {
        for (i = 0; i < R.ntracks; i++) {
            if (!R.tracks[i].self && R.tracks[i].client_id == client_id) {
                h->idx = i;
                h->gen = R.gen;
                found = 1;
                break;
            }
        }
    }
    LeaveCriticalSection(&R.lock);
    return found;
}

int rec_find_self(uint64_t schid, RecHandle *h) {
    int i, found = 0;
    EnterCriticalSection(&R.lock);
    if (R.active && R.schid == schid) {
        for (i = 0; i < R.ntracks; i++) {
            if (R.tracks[i].self) {
                h->idx = i;
                h->gen = R.gen;
                found = 1;
                break;
            }
        }
    }
    LeaveCriticalSection(&R.lock);
    return found;
}

int rec_add_track(uint64_t schid, const RecTrackInfo *info, RecHandle *h) {
    int i, ok = 0;
    EnterCriticalSection(&R.lock);
    if (R.active && R.schid == schid) {
        for (i = 0; i < R.ntracks; i++) {
            if (strcmp(R.tracks[i].uid, info->uid) == 0) {
                R.tracks[i].client_id = info->client_id;
                h->idx = i;
                h->gen = R.gen;
                ok = 1;
                break;
            }
        }
        if (!ok && R.ntracks < CORE_MAX_TRACKS) {
            Track *t = &R.tracks[R.ntracks];
            wchar_t wbase[160], path[DIR_MAX];
            memset(t, 0, sizeof(*t));
            snprintf(t->uid, sizeof(t->uid), "%s", info->uid);
            snprintf(t->nick, sizeof(t->nick), "%s", info->nickname);
            core_track_base(t->base, sizeof(t->base), R.ntracks + 1, info->nickname);
            t->client_id = info->client_id;
            t->self = info->self;
            rec_utf8_to_wide(t->base, wbase, 160);
            _snwprintf(path, DIR_MAX, L"%ls\\%ls.pcm", R.dir, wbase);
            path[DIR_MAX - 1] = 0;
            t->pcm = _wfopen(path, L"wb");
            _snwprintf(path, DIR_MAX, L"%ls\\%ls.idx", R.dir, wbase);
            path[DIR_MAX - 1] = 0;
            t->idx = _wfopen(path, L"wb");
            if (t->pcm && t->idx) {
                setvbuf(t->pcm, NULL, _IOFBF, 65536);
                h->idx = R.ntracks++;
                h->gen = R.gen;
                R.dirty = 1;
                ok = 1;
            } else {
                if (t->pcm) fclose(t->pcm);
                if (t->idx) fclose(t->idx);
                memset(t, 0, sizeof(*t));
            }
        }
    }
    LeaveCriticalSection(&R.lock);
    return ok;
}

void rec_write(const RecHandle *h, const short *samples, int count, int channels) {
    LARGE_INTEGER q;
    Track *t;
    CorePlace pl;
    int64_t now;
    if (count <= 0 || channels <= 0) return;
    EnterCriticalSection(&R.lock);
    if (!R.active || h->gen != R.gen || h->idx < 0 || h->idx >= R.ntracks) {
        LeaveCriticalSection(&R.lock);
        return;
    }
    t = &R.tracks[h->idx];
    QueryPerformanceCounter(&q);
    now = (q.QuadPart - R.qpc0.QuadPart) * CORE_SAMPLE_RATE / R.freq.QuadPart;
    core_place_chunk(&t->run, now, count, &pl);
    if (pl.closed) {
        char line[64];
        core_idx_line(line, sizeof(line), pl.closed_start, pl.closed_count);
        fputs(line, t->idx);
        fflush(t->idx);
        fflush(t->pcm);
    }
    if (channels == 1) {
        fwrite(samples, sizeof(short), (size_t)count, t->pcm);
    } else {
        short buf[1024];
        int done = 0;
        while (done < count) {
            int n = count - done < 1024 ? count - done : 1024, i;
            for (i = 0; i < n; i++) buf[i] = samples[(size_t)(done + i) * (size_t)channels];
            fwrite(buf, sizeof(short), (size_t)n, t->pcm);
            done += n;
        }
    }
    LeaveCriticalSection(&R.lock);
}

void rec_update_live(const CoreLive *live) {
    EnterCriticalSection(&R.lock);
    if (R.active) {
        if (live->map[0]) snprintf(R.map, sizeof(R.map), "%s", live->map);
        if (live->steamid[0]) snprintf(R.steamid, sizeof(R.steamid), "%s", live->steamid);
        if (live->session[0]) snprintf(R.session, sizeof(R.session), "%s", live->session);
        if (live->score_ct >= 0 && live->score_t >= 0) {
            R.score_ct = live->score_ct;
            R.score_t = live->score_t;
        }
        core_rounds_merge(R.rounds, &R.nrounds, CORE_MAX_ROUNDS, live->rounds, live->nrounds);
    }
    LeaveCriticalSection(&R.lock);
}

void rec_tick(uint64_t now_tick_ms) {
    char *json = NULL;
    wchar_t dir[DIR_MAX];
    EnterCriticalSection(&R.lock);
    if (R.active && (R.dirty || now_tick_ms - R.last_manifest_ms >= MANIFEST_EVERY_MS)) {
        R.dirty = 0;
        R.last_manifest_ms = now_tick_ms;
        wcscpy(dir, R.dir);
        json = manifest_json_locked(0, 0);
    }
    LeaveCriticalSection(&R.lock);
    if (json) {
        write_file_atomic(dir, json);
        free(json);
    }
}

static char *read_small_file(const wchar_t *path) {
    FILE *f = _wfopen(path, L"rb");
    char *buf;
    size_t n;
    if (!f) return NULL;
    buf = (char *)malloc(1 << 20);
    if (!buf) {
        fclose(f);
        return NULL;
    }
    n = fread(buf, 1, (1 << 20) - 1, f);
    fclose(f);
    buf[n] = 0;
    return buf;
}

static uint64_t ft_to_u64(const FILETIME *ft) { return ((uint64_t)ft->dwHighDateTime << 32) | ft->dwLowDateTime; }

static void recover_folder(const wchar_t *dir, int64_t now_ms) {
    wchar_t mpath[DIR_MAX], pattern[DIR_MAX];
    WIN32_FIND_DATAW fd;
    HANDLE h;
    char *text, *fixed;
    uint64_t newest = 0, manifest_time = 0;
    int64_t started = 0, ended;
    _snwprintf(mpath, DIR_MAX, L"%ls\\manifest.json", dir);
    mpath[DIR_MAX - 1] = 0;
    text = read_small_file(mpath);
    if (!text) return;
    if (!strstr(text, "\"state\":\"recording\"")) {
        free(text);
        return;
    }
    _snwprintf(pattern, DIR_MAX, L"%ls\\*", dir);
    pattern[DIR_MAX - 1] = 0;
    h = FindFirstFileW(pattern, &fd);
    if (h != INVALID_HANDLE_VALUE) {
        do {
            uint64_t t;
            if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) continue;
            t = ft_to_u64(&fd.ftLastWriteTime);
            if (wcscmp(fd.cFileName, L"manifest.json") == 0) manifest_time = t;
            if (t > newest) newest = t;
        } while (FindNextFileW(h, &fd));
        FindClose(h);
    }
    ended = core_filetime_to_unix_ms(newest);
    if (manifest_time && now_ms - core_filetime_to_unix_ms(manifest_time) < RECOVER_MIN_AGE_MS) {
        free(text);
        return;
    }
    if (core_json_int64(text, "started_at_ms", &started) && ended < started) ended = started;
    fixed = core_manifest_finalize(text, ended);
    free(text);
    if (fixed) {
        write_file_atomic(dir, fixed);
        free(fixed);
    }
}

void rec_recover(const wchar_t *inbox) {
    wchar_t pattern[DIR_MAX], dir[DIR_MAX];
    WIN32_FIND_DATAW fd;
    HANDLE h;
    int64_t now_ms = now_unix_ms();
    _snwprintf(pattern, DIR_MAX, L"%ls\\*", inbox);
    pattern[DIR_MAX - 1] = 0;
    h = FindFirstFileW(pattern, &fd);
    if (h == INVALID_HANDLE_VALUE) return;
    do {
        if (!(fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY)) continue;
        if (wcscmp(fd.cFileName, L".") == 0 || wcscmp(fd.cFileName, L"..") == 0) continue;
        _snwprintf(dir, DIR_MAX, L"%ls\\%ls", inbox, fd.cFileName);
        dir[DIR_MAX - 1] = 0;
        EnterCriticalSection(&R.lock);
        {
            int mine = R.active && wcscmp(R.dir, dir) == 0;
            LeaveCriticalSection(&R.lock);
            if (mine) continue;
        }
        recover_folder(dir, now_ms);
    } while (FindNextFileW(h, &fd));
    FindClose(h);
}
