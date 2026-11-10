/* Loopback poll of Studio's live state from a background thread. */
#ifndef CLIPHUB_POLL_H
#define CLIPHUB_POLL_H

#include <stddef.h>

#include "core.h"

/* Called on the poll thread. ok = 0 means Studio gave no usable answer (live is NULL). */
typedef void (*PollSink)(int ok, const CoreLive *live);

/* Starts the thread; studio_ini is re-read on every cycle. Returns 1 on success. */
int poll_start(const wchar_t *studio_ini, PollSink sink);
/* Signals the thread and waits for it. Safe to call when not running. */
void poll_stop(void);

#endif
