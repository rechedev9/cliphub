#include "core.h"

#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ---- State machine ---- */

void core_sm_init(CoreSm *sm) { memset(sm, 0, sizeof(*sm)); }

static void copy_str(char *dst, size_t cap, const char *src) {
    size_t n = strlen(src);
    if (n >= cap) n = cap - 1;
    memcpy(dst, src, n);
    dst[n] = 0;
}

CoreResult core_sm_step(CoreSm *sm, CoreEvent ev, const char *session) {
    CoreResult r = {CORE_ACT_NONE, CORE_TRIG_AUTO};
    if (!session) session = "";
    switch (ev) {
    case CORE_EV_ENABLE:
        sm->enabled = 1;
        break;
    case CORE_EV_DISABLE:
        sm->enabled = 0;
        sm->latched = 0;
        if (sm->recording) {
            sm->recording = 0;
            r.action = CORE_ACT_STOP;
        }
        break;
    case CORE_EV_LIVE_START:
        sm->live = 1;
        copy_str(sm->live_session, sizeof(sm->live_session), session);
        if (!sm->enabled) break;
        if (!sm->recording) {
            if (sm->latched && strcmp(sm->latch_session, session) == 0) break;
            sm->latched = 0;
            sm->recording = 1;
            sm->trigger = CORE_TRIG_AUTO;
            copy_str(sm->rec_session, sizeof(sm->rec_session), session);
            r.action = CORE_ACT_START;
            r.trigger = CORE_TRIG_AUTO;
        } else if (sm->trigger == CORE_TRIG_AUTO && strcmp(sm->rec_session, session) != 0) {
            sm->latched = 0;
            copy_str(sm->rec_session, sizeof(sm->rec_session), session);
            r.action = CORE_ACT_RESTART;
            r.trigger = CORE_TRIG_AUTO;
        }
        break;
    case CORE_EV_LIVE_END:
        sm->live = 0;
        sm->live_session[0] = 0;
        sm->latched = 0;
        if (sm->enabled && sm->recording && sm->trigger == CORE_TRIG_AUTO) {
            sm->recording = 0;
            r.action = CORE_ACT_STOP;
        }
        break;
    case CORE_EV_STUDIO_LOST:
        if (sm->enabled && sm->recording && sm->trigger == CORE_TRIG_AUTO) {
            sm->recording = 0;
            r.action = CORE_ACT_STOP;
        }
        break;
    case CORE_EV_HOTKEY:
        if (!sm->enabled) break;
        if (!sm->recording) {
            sm->latched = 0;
            sm->recording = 1;
            sm->trigger = CORE_TRIG_MANUAL;
            sm->rec_session[0] = 0;
            r.action = CORE_ACT_START;
            r.trigger = CORE_TRIG_MANUAL;
        } else {
            /* Stopping by hand must not restart on the same live session. */
            const char *s = sm->trigger == CORE_TRIG_AUTO ? sm->rec_session : (sm->live ? sm->live_session : "");
            if (s[0]) {
                sm->latched = 1;
                copy_str(sm->latch_session, sizeof(sm->latch_session), s);
            }
            sm->recording = 0;
            r.action = CORE_ACT_STOP;
        }
        break;
    }
    return r;
}

void core_sm_recording_ended(CoreSm *sm) { sm->recording = 0; }

/* ---- Timing ---- */

void core_place_chunk(CoreRun *run, int64_t now, int count, CorePlace *out) {
    int64_t start = now - count;
    memset(out, 0, sizeof(*out));
    if (run->open && start - run->end <= CORE_JITTER_TOLERANCE) {
        out->start = run->end;
    } else {
        int64_t at = start > run->end ? start : run->end;
        if (run->open) {
            out->closed = 1;
            out->closed_start = run->start;
            out->closed_count = run->end - run->start;
        }
        run->open = 1;
        run->start = at;
        run->end = at;
        out->start = at;
    }
    run->end += count;
}

int core_run_close(CoreRun *run, int64_t *start, int64_t *count) {
    if (!run->open) return 0;
    *start = run->start;
    *count = run->end - run->start;
    run->open = 0;
    return 1;
}

/* ---- Names, folders, idx ---- */

size_t core_sanitize_name(const char *nick, char *out, size_t cap) {
    size_t n = 0, i;
    for (i = 0; nick && nick[i] && n < cap - 1; i++) {
        unsigned char c = (unsigned char)nick[i];
        int bad = c < 0x20 || c == 0x7f || strchr("<>:\"/\\|?*", c) != NULL;
        out[n++] = bad ? '_' : (char)c;
    }
    out[n] = 0;
    if (n > CORE_NAME_MAX_BYTES) {
        n = CORE_NAME_MAX_BYTES;
        while (n > 0 && ((unsigned char)out[n] & 0xC0) == 0x80) n--;
        out[n] = 0;
    }
    while (n > 0 && (out[n - 1] == '.' || out[n - 1] == ' ')) out[--n] = 0;
    if (n == 0) {
        memcpy(out, "voz", 4);
        n = 3;
    }
    return n;
}

void core_track_base(char *out, size_t cap, int index1, const char *nick) {
    char name[CORE_NAME_MAX_BYTES + 8];
    core_sanitize_name(nick, name, sizeof(name));
    snprintf(out, cap, "%02d-%s", index1, name);
}

static void civil_from_days(int64_t z, int *y, int *m, int *d) {
    int64_t era, doe, yoe, doy, mp, yy;
    z += 719468;
    era = (z >= 0 ? z : z - 146096) / 146097;
    doe = z - era * 146097;
    yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    yy = yoe + era * 400;
    doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    mp = (5 * doy + 2) / 153;
    *d = (int)(doy - (153 * mp + 2) / 5 + 1);
    *m = (int)(mp < 10 ? mp + 3 : mp - 9);
    *y = (int)(yy + (*m <= 2));
}

void core_folder_name(char *out, size_t cap, int64_t unix_ms, unsigned rnd16) {
    int64_t secs = unix_ms / 1000;
    int64_t days = secs / 86400;
    int sod = (int)(secs % 86400);
    int y, m, d;
    civil_from_days(days, &y, &m, &d);
    snprintf(out, cap, "%04d%02d%02d-%02d%02d%02d-%04x", y, m, d, sod / 3600, (sod / 60) % 60, sod % 60, rnd16 & 0xffffu);
}

int core_idx_line(char *out, size_t cap, int64_t start, int64_t count) {
    return snprintf(out, cap, "%lld %lld\n", (long long)start, (long long)count);
}

/* ---- Live response ---- */

static int is_word_char(char c, int allow_dash) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || (allow_dash && c == '-');
}

static int valid_word(const char *s, size_t maxlen) {
    size_t i, n = strlen(s);
    if (n == 0 || n > maxlen) return 0;
    for (i = 0; i < n; i++)
        if (!is_word_char(s[i], 1)) return 0;
    return 1;
}

static int parse_uint(const char *s, int64_t *out) {
    int64_t v = 0;
    size_t i, n = strlen(s);
    if (n == 0 || n > 18) return 0;
    for (i = 0; i < n; i++) {
        if (s[i] < '0' || s[i] > '9') return 0;
        v = v * 10 + (s[i] - '0');
    }
    *out = v;
    return 1;
}

static int split_tokens(char *line, char **tok, int max) {
    int n = 0;
    char *p = line;
    while (*p && n < max) {
        while (*p == ' ' || *p == '\t' || *p == '\r') p++;
        if (!*p) break;
        tok[n++] = p;
        while (*p && *p != ' ' && *p != '\t' && *p != '\r') p++;
        if (*p) *p++ = 0;
    }
    return n;
}

void core_rounds_merge(CoreRound *dst, int *ndst, int cap, const CoreRound *src, int nsrc) {
    int i, j, k;
    for (i = 0; i < nsrc; i++) {
        for (j = 0; j < *ndst && dst[j].number != src[i].number; j++) {}
        if (j < *ndst) {
            dst[j].live_at_ms = src[i].live_at_ms;
            continue;
        }
        if (*ndst >= cap) continue;
        for (k = *ndst; k > 0 && dst[k - 1].number > src[i].number; k--) dst[k] = dst[k - 1];
        dst[k] = src[i];
        (*ndst)++;
    }
}

void core_live_parse(const char *text, CoreLive *out) {
    const char *p = text ? text : "";
    memset(out, 0, sizeof(*out));
    out->score_ct = -1;
    out->score_t = -1;
    while (*p) {
        char line[256];
        char *tok[4];
        int nt;
        size_t len = 0;
        const char *e = p;
        while (*e && *e != '\n') e++;
        len = (size_t)(e - p);
        if (len < sizeof(line)) {
            memcpy(line, p, len);
            line[len] = 0;
            nt = split_tokens(line, tok, 4);
            if (nt == 2 && strcmp(tok[0], "live") == 0) {
                if (strcmp(tok[1], "1") == 0) out->live = 1;
                else if (strcmp(tok[1], "0") == 0) out->live = 0;
            } else if (nt == 2 && strcmp(tok[0], "session") == 0) {
                if (valid_word(tok[1], sizeof(out->session) - 1)) copy_str(out->session, sizeof(out->session), tok[1]);
            } else if (nt == 2 && strcmp(tok[0], "map") == 0) {
                if (valid_word(tok[1], sizeof(out->map) - 1)) copy_str(out->map, sizeof(out->map), tok[1]);
            } else if (nt == 2 && strcmp(tok[0], "steamid") == 0) {
                int64_t v;
                if (strlen(tok[1]) <= 20 && parse_uint(tok[1], &v)) copy_str(out->steamid, sizeof(out->steamid), tok[1]);
            } else if (nt == 3 && strcmp(tok[0], "score") == 0) {
                int64_t a, b;
                if (parse_uint(tok[1], &a) && parse_uint(tok[2], &b) && a < 1000 && b < 1000) {
                    out->score_ct = (int)a;
                    out->score_t = (int)b;
                }
            } else if (nt == 3 && strcmp(tok[0], "round") == 0) {
                int64_t n, ms;
                if (parse_uint(tok[1], &n) && parse_uint(tok[2], &ms) && n >= 1 && n < 10000 && ms > 0) {
                    CoreRound r;
                    r.number = (int)n;
                    r.live_at_ms = ms;
                    core_rounds_merge(out->rounds, &out->nrounds, CORE_MAX_ROUNDS, &r, 1);
                }
            }
        }
        p = *e ? e + 1 : e;
    }
}

/* ---- ini and origin ---- */

static char *trim(char *s) {
    char *e;
    while (*s == ' ' || *s == '\t' || *s == '\r') s++;
    e = s + strlen(s);
    while (e > s && (e[-1] == ' ' || e[-1] == '\t' || e[-1] == '\r')) *--e = 0;
    return s;
}

int core_ini_get(const char *text, const char *key, char *out, size_t cap) {
    const char *p = text ? text : "";
    int found = 0;
    if ((unsigned char)p[0] == 0xEF && (unsigned char)p[1] == 0xBB && (unsigned char)p[2] == 0xBF) p += 3;
    while (*p) {
        char line[512];
        char *eq, *k, *v;
        size_t len;
        const char *e = p;
        while (*e && *e != '\n') e++;
        len = (size_t)(e - p);
        if (len < sizeof(line)) {
            memcpy(line, p, len);
            line[len] = 0;
            k = trim(line);
            eq = strchr(k, '=');
            if (k[0] && k[0] != '#' && eq) {
                *eq = 0;
                v = trim(eq + 1);
                k = trim(k);
                if (strcmp(k, key) == 0 && strlen(v) < cap) {
                    memcpy(out, v, strlen(v) + 1);
                    found = 1;
                }
            }
        }
        p = *e ? e + 1 : e;
    }
    return found;
}

static int ieq_prefix(const char *s, const char *prefix) {
    for (; *prefix; s++, prefix++) {
        char a = *s, b = *prefix;
        if (a >= 'A' && a <= 'Z') a = (char)(a + 32);
        if (a != b) return 0;
    }
    return 1;
}

int core_parse_origin(const char *origin, char *host, size_t hostcap, int *port) {
    const char *p, *h;
    size_t hl;
    long v = 0;
    int digits = 0;
    if (!origin || !ieq_prefix(origin, "http://")) return 0;
    p = origin + 7;
    if (strncmp(p, "[::1]:", 6) == 0) {
        h = "::1";
        p += 6;
    } else if (ieq_prefix(p, "127.0.0.1:")) {
        h = "127.0.0.1";
        p += 10;
    } else if (ieq_prefix(p, "localhost:")) {
        h = "localhost";
        p += 10;
    } else {
        return 0;
    }
    while (*p >= '0' && *p <= '9' && digits < 6) {
        v = v * 10 + (*p - '0');
        p++;
        digits++;
    }
    if (*p == '/') p++;
    if (digits == 0 || *p != 0 || v < 1 || v > 65535) return 0;
    hl = strlen(h);
    if (hl >= hostcap) return 0;
    memcpy(host, h, hl + 1);
    *port = (int)v;
    return 1;
}

/* ---- Manifest ---- */

typedef struct {
    char *p;
    size_t len, cap;
    int oom;
} Sb;

static void sb_put(Sb *b, const char *s, size_t n) {
    if (b->oom) return;
    if (b->len + n + 1 > b->cap) {
        size_t nc = b->cap ? b->cap : 1024;
        char *np;
        while (nc < b->len + n + 1) nc *= 2;
        np = (char *)realloc(b->p, nc);
        if (!np) {
            b->oom = 1;
            return;
        }
        b->p = np;
        b->cap = nc;
    }
    memcpy(b->p + b->len, s, n);
    b->len += n;
    b->p[b->len] = 0;
}

static void sb_puts(Sb *b, const char *s) { sb_put(b, s, strlen(s)); }

static void sb_printf(Sb *b, const char *fmt, ...) {
    char tmp[128];
    va_list ap;
    va_start(ap, fmt);
    vsnprintf(tmp, sizeof(tmp), fmt, ap);
    va_end(ap);
    sb_puts(b, tmp);
}

static int utf8_seq_len(const unsigned char *s) {
    unsigned char c = s[0];
    if (c < 0x80) return 1;
    if (c >= 0xC2 && c <= 0xDF) return (s[1] & 0xC0) == 0x80 ? 2 : 0;
    if (c >= 0xE0 && c <= 0xEF) {
        unsigned char lo = c == 0xE0 ? 0xA0 : 0x80, hi = c == 0xED ? 0x9F : 0xBF;
        return s[1] >= lo && s[1] <= hi && (s[2] & 0xC0) == 0x80 ? 3 : 0;
    }
    if (c >= 0xF0 && c <= 0xF4) {
        unsigned char lo = c == 0xF0 ? 0x90 : 0x80, hi = c == 0xF4 ? 0x8F : 0xBF;
        return s[1] >= lo && s[1] <= hi && (s[2] & 0xC0) == 0x80 && (s[3] & 0xC0) == 0x80 ? 4 : 0;
    }
    return 0;
}

static void sb_json_str(Sb *b, const char *s) {
    const unsigned char *u = (const unsigned char *)s;
    sb_put(b, "\"", 1);
    while (*u) {
        int n;
        switch (*u) {
        case '"': sb_puts(b, "\\\""); u++; continue;
        case '\\': sb_puts(b, "\\\\"); u++; continue;
        case '\b': sb_puts(b, "\\b"); u++; continue;
        case '\f': sb_puts(b, "\\f"); u++; continue;
        case '\n': sb_puts(b, "\\n"); u++; continue;
        case '\r': sb_puts(b, "\\r"); u++; continue;
        case '\t': sb_puts(b, "\\t"); u++; continue;
        default: break;
        }
        if (*u < 0x20) {
            sb_printf(b, "\\u%04x", *u);
            u++;
            continue;
        }
        n = utf8_seq_len(u);
        if (n == 0) {
            sb_put(b, "?", 1);
            u++;
        } else {
            sb_put(b, (const char *)u, (size_t)n);
            u += n;
        }
    }
    sb_put(b, "\"", 1);
}

char *core_manifest_json(const CoreManifest *m) {
    Sb b = {0, 0, 0, 0};
    int i;
    sb_printf(&b, "{\"format\":1,\"plugin_version\":\"%s\",\"state\":\"%s\",\"interrupted\":%s,", CORE_PLUGIN_VERSION, m->complete ? "complete" : "recording",
              m->interrupted ? "true" : "false");
    sb_printf(&b, "\"started_at_ms\":%lld,\"ended_at_ms\":%lld,\"sample_rate\":%d,\"trigger\":\"%s\",", (long long)m->started_at_ms, (long long)m->ended_at_ms,
              CORE_SAMPLE_RATE, m->trigger == CORE_TRIG_MANUAL ? "manual" : "auto");
    sb_puts(&b, "\"server\":");
    sb_json_str(&b, m->server);
    sb_puts(&b, ",\"channel\":");
    sb_json_str(&b, m->channel);
    sb_puts(&b, ",\"map\":");
    sb_json_str(&b, m->map);
    sb_puts(&b, ",\"self_steamid64\":");
    sb_json_str(&b, m->steamid);
    sb_puts(&b, ",\"session\":");
    sb_json_str(&b, m->session);
    sb_printf(&b, ",\"score_ct\":%d,\"score_t\":%d,\"rounds\":[", m->score_ct, m->score_t);
    for (i = 0; i < m->nrounds; i++)
        sb_printf(&b, "%s{\"round\":%d,\"live_at_ms\":%lld}", i ? "," : "", m->rounds[i].number, (long long)m->rounds[i].live_at_ms);
    sb_puts(&b, "],\"tracks\":[");
    for (i = 0; i < m->ntracks; i++) {
        if (i) sb_put(&b, ",", 1);
        sb_puts(&b, "{\"file\":");
        sb_json_str(&b, m->tracks[i].file);
        sb_puts(&b, ",\"nickname\":");
        sb_json_str(&b, m->tracks[i].nickname);
        sb_puts(&b, ",\"uid\":");
        sb_json_str(&b, m->tracks[i].uid);
        sb_printf(&b, ",\"self\":%s}", m->tracks[i].self ? "true" : "false");
    }
    sb_puts(&b, "]}\n");
    if (b.oom) {
        free(b.p);
        return NULL;
    }
    return b.p;
}

static char *replace_first(const char *text, const char *from, const char *to) {
    const char *at = strstr(text, from);
    size_t pre, flen = strlen(from), tlen = strlen(to), rest;
    char *out;
    if (!at) return NULL;
    pre = (size_t)(at - text);
    rest = strlen(at + flen);
    out = (char *)malloc(pre + tlen + rest + 1);
    if (!out) return NULL;
    memcpy(out, text, pre);
    memcpy(out + pre, to, tlen);
    memcpy(out + pre + tlen, at + flen, rest + 1);
    return out;
}

char *core_manifest_finalize(const char *text, int64_t ended_at_ms) {
    char ended[64];
    char *a, *b, *c;
    a = replace_first(text, "\"state\":\"recording\"", "\"state\":\"complete\"");
    if (!a) return NULL;
    b = replace_first(a, "\"interrupted\":false", "\"interrupted\":true");
    free(a);
    if (!b) return NULL;
    snprintf(ended, sizeof(ended), "\"ended_at_ms\":%lld,", (long long)ended_at_ms);
    c = replace_first(b, "\"ended_at_ms\":0,", ended);
    free(b);
    return c;
}

int core_json_int64(const char *text, const char *key, int64_t *out) {
    char pat[64];
    const char *at;
    snprintf(pat, sizeof(pat), "\"%s\":", key);
    at = strstr(text, pat);
    if (!at) return 0;
    *out = (int64_t)strtoll(at + strlen(pat), NULL, 10);
    return 1;
}

int64_t core_filetime_to_unix_ms(uint64_t ft) { return (int64_t)((ft - 116444736000000000ULL) / 10000ULL); }
