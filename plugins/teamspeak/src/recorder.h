/* Recording runtime: inbox folder, per-person tracks, manifest. No TS3 headers. */
#ifndef CLIPHUB_RECORDER_H
#define CLIPHUB_RECORDER_H

#include <stddef.h>
#include <stdint.h>

#include "core.h"

typedef struct {
    int idx;
    unsigned gen;
} RecHandle;

typedef struct {
    unsigned client_id;
    const char *uid;
    const char *nickname;
    int self;
} RecTrackInfo;

typedef struct {
    const wchar_t *inbox;
    CoreTrigger trigger;
    const char *server;
    const char *channel;
    uint64_t schid;
} RecStartOpts;

void rec_init(void);
void rec_shutdown(void);

/* Returns 1 when the folder and the first manifest were written. */
int rec_start(const RecStartOpts *opts);
/* Returns the number of tracks, or -1 when nothing was recording. */
int rec_stop(void);
/* Returns 1 and fills schid when a recording is active. */
int rec_active(uint64_t *schid);

int rec_find_track(uint64_t schid, unsigned client_id, RecHandle *h);
int rec_find_self(uint64_t schid, RecHandle *h);
int rec_add_track(uint64_t schid, const RecTrackInfo *info, RecHandle *h);
void rec_write(const RecHandle *h, const short *samples, int count, int channels);

void rec_update_live(const CoreLive *live);
/* Rewrites the manifest when a track appeared or 30 s passed. */
void rec_tick(uint64_t now_tick_ms);

/* Marks manifests left in "recording" by a dead process as complete + interrupted. */
void rec_recover(const wchar_t *inbox);

int rec_utf8_to_wide(const char *s, wchar_t *out, int cap);
int rec_wide_to_utf8(const wchar_t *s, char *out, int cap);
void rec_mkdir_p(const wchar_t *path);

#endif
