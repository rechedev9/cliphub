#include "../src/core.h"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static int g_checks = 0, g_fail = 0, g_tests = 0;

#define CHECK(cond) \
    do { \
        g_checks++; \
        if (!(cond)) { \
            g_fail++; \
            printf("  FAIL %s:%d: %s\n", __FILE__, __LINE__, #cond); \
        } \
    } while (0)
#define CHECK_STR(a, b) \
    do { \
        g_checks++; \
        if (strcmp((a), (b)) != 0) { \
            g_fail++; \
            printf("  FAIL %s:%d: \"%s\" != \"%s\"\n", __FILE__, __LINE__, (a), (b)); \
        } \
    } while (0)
#define RUN(fn) \
    do { \
        g_tests++; \
        printf("%s\n", #fn); \
        fn(); \
    } while (0)

static CoreResult step(CoreSm *sm, CoreEvent ev, const char *s) { return core_sm_step(sm, ev, s); }

static void test_sm_disabled(void) {
    CoreSm sm;
    core_sm_init(&sm);
    CHECK(step(&sm, CORE_EV_HOTKEY, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "a").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_END, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_STUDIO_LOST, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_DISABLE, "").action == CORE_ACT_NONE);
}

static void test_sm_auto(void) {
    CoreSm sm;
    CoreResult r;
    core_sm_init(&sm);
    step(&sm, CORE_EV_ENABLE, "");
    r = step(&sm, CORE_EV_LIVE_START, "s1");
    CHECK(r.action == CORE_ACT_START && r.trigger == CORE_TRIG_AUTO);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_NONE);
    r = step(&sm, CORE_EV_LIVE_START, "s2");
    CHECK(r.action == CORE_ACT_RESTART && r.trigger == CORE_TRIG_AUTO);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_END, "").action == CORE_ACT_STOP);
    CHECK(step(&sm, CORE_EV_LIVE_END, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s3").action == CORE_ACT_START);
    CHECK(step(&sm, CORE_EV_STUDIO_LOST, "").action == CORE_ACT_STOP);
    CHECK(step(&sm, CORE_EV_STUDIO_LOST, "").action == CORE_ACT_NONE);
}

static void test_sm_disable_while_recording(void) {
    CoreSm sm;
    core_sm_init(&sm);
    step(&sm, CORE_EV_ENABLE, "");
    step(&sm, CORE_EV_LIVE_START, "s1");
    CHECK(step(&sm, CORE_EV_DISABLE, "").action == CORE_ACT_STOP);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_NONE);
    step(&sm, CORE_EV_ENABLE, "");
    CHECK(step(&sm, CORE_EV_HOTKEY, "").action == CORE_ACT_START);
    CHECK(step(&sm, CORE_EV_DISABLE, "").action == CORE_ACT_STOP);
}

static void test_sm_hotkey_latch(void) {
    CoreSm sm;
    CoreResult r;
    core_sm_init(&sm);
    step(&sm, CORE_EV_ENABLE, "");
    step(&sm, CORE_EV_LIVE_START, "s1");
    CHECK(step(&sm, CORE_EV_HOTKEY, "").action == CORE_ACT_STOP);
    /* The poll keeps saying live: the latch holds for the same session. */
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_NONE);
    /* A new session clears the latch. */
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_START);
    step(&sm, CORE_EV_HOTKEY, "");
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_NONE);
    /* LIVE_END clears the latch, even for the same session id. */
    step(&sm, CORE_EV_LIVE_END, "");
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_START);
    /* A hotkey while idle starts a manual recording. */
    step(&sm, CORE_EV_HOTKEY, "");
    r = step(&sm, CORE_EV_HOTKEY, "");
    CHECK(r.action == CORE_ACT_START && r.trigger == CORE_TRIG_MANUAL);
}

static void test_sm_manual(void) {
    CoreSm sm;
    core_sm_init(&sm);
    step(&sm, CORE_EV_ENABLE, "");
    CHECK(step(&sm, CORE_EV_HOTKEY, "").action == CORE_ACT_START);
    CHECK(step(&sm, CORE_EV_LIVE_END, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_STUDIO_LOST, "").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_HOTKEY, "").action == CORE_ACT_STOP);
    /* Stopped by hand during a live match: no automatic restart for it. */
    CHECK(step(&sm, CORE_EV_LIVE_START, "s2").action == CORE_ACT_NONE);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s3").action == CORE_ACT_START);
}

static void test_sm_start_failed(void) {
    CoreSm sm;
    core_sm_init(&sm);
    step(&sm, CORE_EV_ENABLE, "");
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_START);
    core_sm_recording_ended(&sm);
    CHECK(step(&sm, CORE_EV_LIVE_START, "s1").action == CORE_ACT_START);
}

static void test_timing(void) {
    CoreRun run;
    CorePlace pl;
    int64_t s, c;
    memset(&run, 0, sizeof(run));
    /* First chunk of 960 arrives at 48000: starts at 47040. */
    core_place_chunk(&run, 48000, 960, &pl);
    CHECK(!pl.closed && pl.start == 47040 && run.start == 47040 && run.end == 48000);
    /* Contiguous. */
    core_place_chunk(&run, 48960, 960, &pl);
    CHECK(!pl.closed && pl.start == 48000 && run.end == 48960);
    /* Jitter inside the tolerance (chunk arrives 2 frames late): still contiguous. */
    core_place_chunk(&run, 48960 + 960 + 1920, 960, &pl);
    CHECK(!pl.closed && pl.start == 48960 && run.end == 49920);
    /* Arrives early (natural start before the end): never overlaps. */
    core_place_chunk(&run, 49920, 960, &pl);
    CHECK(!pl.closed && pl.start == 49920 && run.end == 50880);
    /* Exactly at the tolerance edge: contiguous. */
    core_place_chunk(&run, 50880 + CORE_JITTER_TOLERANCE + 960, 960, &pl);
    CHECK(!pl.closed && pl.start == 50880);
    /* One sample past the tolerance: a gap opens a new run. */
    {
        int64_t end = run.end, startOld = run.start;
        core_place_chunk(&run, end + CORE_JITTER_TOLERANCE + 1 + 960, 960, &pl);
        CHECK(pl.closed && pl.closed_start == startOld && pl.closed_count == end - startOld);
        CHECK(pl.start == end + CORE_JITTER_TOLERANCE + 1 && run.start == pl.start && run.end == pl.start + 960);
    }
    CHECK(core_run_close(&run, &s, &c) && s == run.start && c == 960);
    CHECK(!core_run_close(&run, &s, &c));
    /* After a close the next chunk opens a run no earlier than the last end. */
    core_place_chunk(&run, run.end - 100 + 960, 960, &pl);
    CHECK(!pl.closed && pl.start >= run.start && run.end - run.start == 960);
    {
        CoreRun r2;
        memset(&r2, 0, sizeof(r2));
        core_place_chunk(&r2, 500, 960, &pl); /* arrives before one chunk fits: clamps to 0 */
        CHECK(pl.start == 0 && r2.end == 960);
    }
}

static void test_sanitize(void) {
    char out[64];
    char nick[200];
    int i;
    core_sanitize_name("a<b>c:d\"e/f\\g|h?i*j", out, sizeof(out));
    CHECK_STR(out, "a_b_c_d_e_f_g_h_i_j");
    core_sanitize_name("tab\there\nx\x7f", out, sizeof(out));
    CHECK_STR(out, "tab_here_x_");
    core_sanitize_name("name. . ", out, sizeof(out));
    CHECK_STR(out, "name");
    core_sanitize_name("...", out, sizeof(out));
    CHECK_STR(out, "voz");
    core_sanitize_name("", out, sizeof(out));
    CHECK_STR(out, "voz");
    core_sanitize_name(NULL, out, sizeof(out));
    CHECK_STR(out, "voz");
    core_sanitize_name("Jos\xC3\xA9 Mu\xC3\xB1oz", out, sizeof(out));
    CHECK_STR(out, "Jos\xC3\xA9 Mu\xC3\xB1oz");
    /* 39 ASCII bytes then a 2-byte char: 41 bytes, must not cut the char in half. */
    memset(nick, 'a', 39);
    memcpy(nick + 39, "\xC3\xA9", 3);
    core_sanitize_name(nick, out, sizeof(out));
    CHECK(strlen(out) == 39);
    /* 40 bytes exactly stays. */
    memset(nick, 'b', 38);
    memcpy(nick + 38, "\xC3\xA9", 3);
    core_sanitize_name(nick, out, sizeof(out));
    CHECK(strlen(out) == 40);
    /* Long ASCII is cut at 40 and a trailing space exposed by the cut is dropped. */
    for (i = 0; i < 100; i++) nick[i] = 'x';
    nick[39] = ' ';
    nick[100] = 0;
    core_sanitize_name(nick, out, sizeof(out));
    CHECK(strlen(out) == 39);
    /* 4-byte emoji at the cut. */
    memset(nick, 'c', 38);
    memcpy(nick + 38, "\xF0\x9F\x8E\xAF\xF0\x9F\x8E\xAF", 9);
    core_sanitize_name(nick, out, sizeof(out));
    CHECK(strlen(out) == 38);
    core_track_base(out, sizeof(out), 3, "CON");
    CHECK_STR(out, "03-CON");
    core_track_base(out, sizeof(out), 12, "");
    CHECK_STR(out, "12-voz");
}

static int contains(const char *hay, const char *needle) { return strstr(hay, needle) != NULL; }

static void test_manifest(void) {
    CoreManifest *m = (CoreManifest *)calloc(1, sizeof(*m));
    char *j;
    CoreRound a[2], b[2];
    m->started_at_ms = 1731234567890LL;
    m->trigger = CORE_TRIG_MANUAL;
    m->score_ct = -1;
    m->score_t = -1;
    strcpy(m->server, "Mi \"server\" \\ ES");
    strcpy(m->channel, "Canal\x01\x1f\n\ttab \xC3\xB1\xE2\x82\xAC");
    strcpy(m->session, "3f2a9c");
    m->ntracks = 2;
    strcpy(m->tracks[0].file, "01-nick");
    strcpy(m->tracks[0].nickname, "nick \"q\"");
    strcpy(m->tracks[0].uid, "uid/+=");
    m->tracks[0].self = 1;
    strcpy(m->tracks[1].file, "02-bad");
    strcpy(m->tracks[1].nickname, "bad\xff" "utf");
    a[0].number = 2;
    a[0].live_at_ms = 200;
    a[1].number = 1;
    a[1].live_at_ms = 100;
    core_rounds_merge(m->rounds, &m->nrounds, CORE_MAX_ROUNDS, a, 2);
    b[0].number = 2;
    b[0].live_at_ms = 250;
    b[1].number = 3;
    b[1].live_at_ms = 300;
    core_rounds_merge(m->rounds, &m->nrounds, CORE_MAX_ROUNDS, b, 2);
    CHECK(m->nrounds == 3 && m->rounds[0].number == 1 && m->rounds[1].number == 2 && m->rounds[1].live_at_ms == 250 && m->rounds[2].number == 3);

    j = core_manifest_json(m);
    CHECK(j != NULL);
    if (j) {
        CHECK(contains(j, "{\"format\":1,\"plugin_version\":\"1.0.0\",\"state\":\"recording\",\"interrupted\":false,"));
        CHECK(contains(j, "\"started_at_ms\":1731234567890,\"ended_at_ms\":0,\"sample_rate\":48000,\"trigger\":\"manual\""));
        CHECK(contains(j, "\"server\":\"Mi \\\"server\\\" \\\\ ES\""));
        CHECK(contains(j, "\"channel\":\"Canal\\u0001\\u001f\\n\\ttab \xC3\xB1\xE2\x82\xAC\""));
        CHECK(contains(j, "\"map\":\"\",\"self_steamid64\":\"\",\"session\":\"3f2a9c\",\"score_ct\":-1,\"score_t\":-1"));
        CHECK(contains(j, "\"rounds\":[{\"round\":1,\"live_at_ms\":100},{\"round\":2,\"live_at_ms\":250},{\"round\":3,\"live_at_ms\":300}]"));
        CHECK(contains(j, "{\"file\":\"01-nick\",\"nickname\":\"nick \\\"q\\\"\",\"uid\":\"uid/+=\",\"self\":true}"));
        CHECK(contains(j, "\"nickname\":\"bad?utf\""));
        CHECK(j[strlen(j) - 2] == '}');
        {
            /* No raw control character survives inside the document. */
            size_t i, bad = 0;
            for (i = 0; j[i]; i++)
                if ((unsigned char)j[i] < 0x20 && j[i] != '\n') bad++;
            CHECK(bad == 0);
        }
        free(j);
    }

    m->complete = 1;
    m->ended_at_ms = 1731234999999LL;
    j = core_manifest_json(m);
    CHECK(j && contains(j, "\"state\":\"complete\"") && contains(j, "\"ended_at_ms\":1731234999999,"));
    free(j);

    m->complete = 0;
    m->ended_at_ms = 0;
    j = core_manifest_json(m);
    if (j) {
        char *f = core_manifest_finalize(j, 1731235000000LL);
        int64_t v = 0;
        CHECK(f != NULL);
        if (f) {
            CHECK(contains(f, "\"state\":\"complete\"") && contains(f, "\"interrupted\":true") && contains(f, "\"ended_at_ms\":1731235000000,"));
            CHECK(!contains(f, "\"state\":\"recording\""));
            CHECK(core_json_int64(f, "started_at_ms", &v) && v == 1731234567890LL);
            CHECK(core_manifest_finalize(f, 5) == NULL);
            free(f);
        }
        free(j);
    }
    free(m);
}

static void test_live_parse(void) {
    CoreLive l;
    core_live_parse("live 0\n", &l);
    CHECK(l.live == 0 && l.nrounds == 0 && l.score_ct == -1);
    core_live_parse("", &l);
    CHECK(l.live == 0);
    core_live_parse(NULL, &l);
    CHECK(l.live == 0);
    core_live_parse("live 1\nsession 3f2a9c\nmap de_mirage\nsteamid 76561198000000000\nscore 3 2\nround 1 1731234600000\nround 2 1731234700000\n", &l);
    CHECK(l.live == 1 && l.nrounds == 2 && l.score_ct == 3 && l.score_t == 2);
    CHECK_STR(l.session, "3f2a9c");
    CHECK_STR(l.map, "de_mirage");
    CHECK_STR(l.steamid, "76561198000000000");
    CHECK(l.rounds[0].number == 1 && l.rounds[0].live_at_ms == 1731234600000LL && l.rounds[1].number == 2);
    /* CRLF, unknown keys, malformed lines. */
    core_live_parse("live 1\r\nfoo bar baz\r\nscore x y\r\nscore 5\r\nround 0 5\r\nround a 5\r\nround 4\r\nround 3 77\r\nround 3 88\r\nmap bad/map\r\nsession ok_1\r\nsteamid 12ab\r\nlive 7\r\n", &l);
    CHECK(l.live == 1 && l.score_ct == -1 && l.nrounds == 1 && l.rounds[0].number == 3 && l.rounds[0].live_at_ms == 88);
    CHECK_STR(l.session, "ok_1");
    CHECK_STR(l.map, "");
    CHECK_STR(l.steamid, "");
    /* No trailing newline. */
    core_live_parse("live 1\nsession abc", &l);
    CHECK(l.live == 1);
    CHECK_STR(l.session, "abc");
    /* Garbage without a live line is not a match. */
    core_live_parse("hello world\n", &l);
    CHECK(l.live == 0);
}

static void test_ini_origin(void) {
    char v[64], host[32];
    int port = 0;
    const char *ini = "\xEF\xBB\xBF# comment\r\n\r\n  web_origin =  http://127.0.0.1:43123 \r\nother=1\nweb_origin_x=nope\nenabled=1\n=empty\nnoequals\n";
    CHECK(core_ini_get(ini, "web_origin", v, sizeof(v)));
    CHECK_STR(v, "http://127.0.0.1:43123");
    CHECK(core_ini_get(ini, "enabled", v, sizeof(v)) && strcmp(v, "1") == 0);
    CHECK(!core_ini_get(ini, "missing", v, sizeof(v)));
    CHECK(!core_ini_get("# web_origin=x\n", "web_origin", v, sizeof(v)));
    CHECK(core_ini_get("k=a\nk=b\n", "k", v, sizeof(v)) && strcmp(v, "b") == 0);
    CHECK(!core_ini_get("k=0123456789\n", "k", v, 5));

    CHECK(core_parse_origin("http://127.0.0.1:43123", host, sizeof(host), &port) && port == 43123 && strcmp(host, "127.0.0.1") == 0);
    CHECK(core_parse_origin("http://localhost:80", host, sizeof(host), &port) && port == 80 && strcmp(host, "localhost") == 0);
    CHECK(core_parse_origin("HTTP://LocalHost:8080/", host, sizeof(host), &port) && port == 8080);
    CHECK(core_parse_origin("http://[::1]:5000", host, sizeof(host), &port) && port == 5000 && strcmp(host, "::1") == 0);
    CHECK(!core_parse_origin("https://127.0.0.1:43123", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://192.168.1.5:43123", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://example.com:80", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1.evil.com:80", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://localhost.evil.com:80", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1:", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1:0", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1:65536", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1:80/path", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://127.0.0.1:80@evil.com", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://user@127.0.0.1:80", host, sizeof(host), &port));
    CHECK(!core_parse_origin("http://[::2]:80", host, sizeof(host), &port));
    CHECK(!core_parse_origin("", host, sizeof(host), &port));
    CHECK(!core_parse_origin(NULL, host, sizeof(host), &port));
}

static void test_folder_idx(void) {
    char out[64];
    core_folder_name(out, sizeof(out), 1731234567890LL, 0x0abc);
    CHECK_STR(out, "20241110-102927-0abc");
    core_folder_name(out, sizeof(out), 951865200000LL, 0xffff);
    CHECK_STR(out, "20000229-230000-ffff");
    core_folder_name(out, sizeof(out), 0, 0x1ffff);
    CHECK_STR(out, "19700101-000000-ffff");
    CHECK(core_idx_line(out, sizeof(out), 47040, 960000) == 13);
    CHECK_STR(out, "47040 960000\n");
    core_idx_line(out, sizeof(out), 0, 1);
    CHECK_STR(out, "0 1\n");
    core_idx_line(out, sizeof(out), 5000000000LL, 960);
    CHECK_STR(out, "5000000000 960\n");
    CHECK(core_filetime_to_unix_ms(116444736000000000ULL) == 0);
    CHECK(core_filetime_to_unix_ms(116444736000000000ULL + 17312345678900000ULL) == 1731234567890LL);
}

int main(void) {
    RUN(test_sm_disabled);
    RUN(test_sm_auto);
    RUN(test_sm_disable_while_recording);
    RUN(test_sm_hotkey_latch);
    RUN(test_sm_manual);
    RUN(test_sm_start_failed);
    RUN(test_timing);
    RUN(test_sanitize);
    RUN(test_manifest);
    RUN(test_live_parse);
    RUN(test_ini_origin);
    RUN(test_folder_idx);
    printf("%d tests, %d checks, %d failed\n", g_tests, g_checks, g_fail);
    return g_fail ? 1 : 0;
}
