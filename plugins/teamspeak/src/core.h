/* Pure logic of the ClipHub TeamSpeak plugin: no Windows or TS3 headers. */
#ifndef CLIPHUB_CORE_H
#define CLIPHUB_CORE_H

#include <stddef.h>
#include <stdint.h>

#define CORE_SAMPLE_RATE 48000
#define CORE_JITTER_TOLERANCE 2880 /* 3 frames of 960 samples */
#define CORE_NAME_MAX_BYTES 40
#define CORE_MAX_ROUNDS 256
#define CORE_MAX_TRACKS 32
#define CORE_STR_MAX 128
#define CORE_PLUGIN_VERSION "1.0.0"

/* ---- State machine ---- */
typedef enum { CORE_EV_ENABLE, CORE_EV_DISABLE, CORE_EV_HOTKEY, CORE_EV_LIVE_START, CORE_EV_LIVE_END, CORE_EV_STUDIO_LOST } CoreEvent;
typedef enum { CORE_ACT_NONE, CORE_ACT_START, CORE_ACT_STOP, CORE_ACT_RESTART } CoreAction;
typedef enum { CORE_TRIG_AUTO, CORE_TRIG_MANUAL } CoreTrigger;

typedef struct {
    int enabled;
    int recording;
    CoreTrigger trigger;
    int latched;
    char latch_session[48];
    char rec_session[48]; /* session an auto recording was started for */
    int live;             /* last known live state */
    char live_session[48];
} CoreSm;

typedef struct {
    CoreAction action;
    CoreTrigger trigger; /* meaningful for START and RESTART */
} CoreResult;

void core_sm_init(CoreSm *sm);
CoreResult core_sm_step(CoreSm *sm, CoreEvent ev, const char *session);
/* The glue could not start (or lost) the recording the machine asked for. */
void core_sm_recording_ended(CoreSm *sm);

/* ---- Timing ---- */
typedef struct {
    int open;
    int64_t start; /* open run start, timeline samples */
    int64_t end;   /* end of the last run (open or closed) */
} CoreRun;

typedef struct {
    int closed;           /* a previous run was closed: write its line */
    int64_t closed_start;
    int64_t closed_count;
    int64_t start;        /* where this chunk sits on the timeline */
} CorePlace;

/* now = elapsed samples when the chunk arrived; count = samples in the chunk. */
void core_place_chunk(CoreRun *run, int64_t now, int count, CorePlace *out);
/* Closes the open run, if any. Returns 1 and fills start/count when it did. */
int core_run_close(CoreRun *run, int64_t *start, int64_t *count);

/* ---- Names, folders, idx ---- */
size_t core_sanitize_name(const char *nick, char *out, size_t cap); /* cap >= 41 */
void core_track_base(char *out, size_t cap, int index1, const char *nick);
void core_folder_name(char *out, size_t cap, int64_t unix_ms, unsigned rnd16);
int core_idx_line(char *out, size_t cap, int64_t start, int64_t count);

/* ---- Live response ---- */
typedef struct {
    int number;
    int64_t live_at_ms;
} CoreRound;

typedef struct {
    int live;
    char session[48];
    char map[64];
    char steamid[24];
    int score_ct;
    int score_t;
    int nrounds;
    CoreRound rounds[CORE_MAX_ROUNDS];
} CoreLive;

void core_live_parse(const char *text, CoreLive *out);
void core_rounds_merge(CoreRound *dst, int *ndst, int cap, const CoreRound *src, int nsrc);

/* ---- ini and origin ---- */
int core_ini_get(const char *text, const char *key, char *out, size_t cap);
/* Accepts http://127.0.0.1:port, http://localhost:port, http://[::1]:port only. */
int core_parse_origin(const char *origin, char *host, size_t hostcap, int *port);

/* ---- Manifest ---- */
typedef struct {
    char file[CORE_STR_MAX];
    char nickname[CORE_STR_MAX];
    char uid[CORE_STR_MAX];
    int self;
} CoreManifestTrack;

typedef struct {
    int complete;
    int interrupted;
    int64_t started_at_ms;
    int64_t ended_at_ms;
    CoreTrigger trigger;
    char server[CORE_STR_MAX];
    char channel[CORE_STR_MAX];
    char map[64];
    char steamid[24];
    char session[48];
    int score_ct;
    int score_t;
    int nrounds;
    CoreRound rounds[CORE_MAX_ROUNDS];
    int ntracks;
    CoreManifestTrack tracks[CORE_MAX_TRACKS];
} CoreManifest;

/* Returns a malloc'd NUL-terminated UTF-8 string, or NULL when out of memory. */
char *core_manifest_json(const CoreManifest *m);
/* Rewrites a "recording" manifest text as complete + interrupted. NULL if it is not recording. */
char *core_manifest_finalize(const char *text, int64_t ended_at_ms);
int core_json_int64(const char *text, const char *key, int64_t *out);

int64_t core_filetime_to_unix_ms(uint64_t filetime100ns);

#endif
