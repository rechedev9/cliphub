---
status: accepted
---

# Stream subtítulos also come from a mixed track, without the measured threshold

[ADR 0001](0001-stream-subtitulos-restart-local-mic-first.md) let a stream with only a mixed track have subtítulos only once that case passed a measured threshold on hand-corrected streams. No stream was ever hand-corrected, so the gate left out every VOD imported by link. The owner asked for subtítulos on those streams now, so a mixed track is transcribed like a mic track, from its only audio track, with the same Parakeet install and the same speech detector.

Before the change we ran the pinned Parakeet on five minutes of a real Twitch VOD with one mixed track. The streamer's Spanish came out readable. Most of the wrong frases were teammates' English callouts and the CS2 announcer. Sending the whole stretch to Parakeet without the speech detector gave fewer words and dropped whole stretches, so the detector stays.

## Consequences

- `availability` still says `mixed_track`, and the Subtítulos panel tells the user that teammates and the game can come in and that they should discard those frases.
- A render no longer goes without subtítulos because the track is mixed.
- The measured threshold of ADR 0001 no longer gates anything. `zv transcript measure` stays the way to measure mixed tracks once they are corrected by hand.
