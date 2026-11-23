---
status: accepted, its threshold for a mixed track superseded by ADR 0002
---

# Stream subtítulos restart from a new design: local, mic track first

Stream subtítulos were removed in July 2026 (commit 77a9534) because Studio's transcripción of the full stream mix, made in the xAI cloud and translated to Spanish, was not good enough to ship and its review gate was heavy to keep. We bring them back with a new design instead of restoring that code: the transcripción runs only on the PC, with the same pinned Parakeet install the demo voice uses, on the mic track when the stream has one. A stream with only a mixed track is transcribed too, but that case ships only once it passes a measured threshold on hand-corrected streams.

## Considered options

- **Restore the July pipeline.** Rejected: the design doc of the removal (commit 1a677910) says a retry should start from a fresh design, and Studio's pipeline sent audio off the PC.
- **A cloud speech service.** Rejected: Studio keeps the user's recordings local, and a second engine doubles what must be measured.
- **Clean or separate the mix before transcribing.** Rejected for now: the studies read for this decision report that enhancement lowers recognition accuracy more often than it raises it. We have not measured it ourselves, so it can return behind the same measurement.
- **Mic track only.** Rejected: a VOD imported by link has one mixed track, so this leaves out most streams.

## Consequences

- Subtítulos are in the language spoken; there is no translation.
- A stream has no speaker names and no speaker separation: every frase counts as the streamer's.
- Only the stretches that go into a video are transcribed, so a stream cannot be searched by text.
- A stream render does not require a review of the frases.
