# Full Demo AAC recovery

Zack's saved 705.316667-second program failed after every capture and video
assembly step had completed. The native AAC encoder reconstructed peaks above
the approved -1.5 dBTP ceiling. Lowering loudnorm's peak target then reduced the
whole mix, preventing the existing three attempts from reaching -14 LUFS.

The three original mastering attempts remain first. Their successful media is
unchanged. After all three fail, Windows builds with Media Foundation AAC can
try up to three additional masters from the original lossless mixed program.
The initial normalization stays fixed; a post-normalization oversampled limiter
and bounded gain correction independently control peaks and loudness. Automatic
limiter gain is disabled and its latency is compensated. Each recovery attempt
records the encoder, gain, ceiling, and decoded AAC measurements.

The composed loudnorm/limiter pass also produced AAC with a reported stream
duration about 90 ms beyond the source end. Recovery now bounds its lossless
audio output to the approved sample count and rebuilds its timestamps from that
clock before encoding. Bounding samples alone did not fix the duration. The
final AAC packet's duration excludes encoder padding, matching native AAC's
short final packet metadata; otherwise some frame counts exceeded the delivery
tolerance by up to one AAC packet. The regression uses the canonical
video frame count for duration; a NUT container's reported duration can be the
timestamp of its last packet rather than the end of its final frame.

Delivery still requires decoded loudness within 0.5 LU of -14 LUFS, true peak at
or below -1.5 dBTP, stereo AAC at 48 kHz, the approved duration, and a complete
video/audio decode. There is no acceptance fallback. An unavailable encoder or
three unsuccessful recovery attempts remains an error. Exhausted attempts now
include the measured LUFS and true peaks in the error retained by the worker.

The shipped FFmpeg n8.1.2-30-g45f1910444-20260723 exposes `aac_mf`. The saved real
audio, encoded with its first recovery settings, measured -13.90 LUFS and
-2.87 dBTP, with an LRA of 8.30 LU. After bounding packet padding, the delivered
audio and video both last 705.316667 seconds. A generated impulse also retained
its original sample position after Media Foundation encoding and decoding.
The original native attempts measured
-15.19/+2.99, -19.34/-1.47, and -19.81/-0.26 (LUFS/dBTP).

Validation includes existing ordinary/silent media canaries, a synthetic
1080p60 recovery and full delivery check, contiguous AAC packet timestamps and
an exact final packet boundary, bounded corrections, and an opt-in
saved-program regression. The latter must reproduce all three native failures,
pass recovery, preserve the decoded video hash, and keep audio duration within
one video frame. The private game/comms recording is not a repository fixture.
Set `FULL_DEMO_MASTER_REGRESSION_INPUT` to its lossless A/V reproduction and
optionally `FULL_DEMO_EVIDENCE_DIR` to retain the measurement evidence.

This does not establish that every possible source will meet both audio
constraints. Sources that still fail are rejected and leave capture artifacts
available for another render. The recovery does not run on platforms without
Media Foundation AAC.

References: [FFmpeg Media Foundation encoders](https://ffmpeg.org/ffmpeg-codecs.html#MediaFoundation),
[Microsoft AAC encoder](https://learn.microsoft.com/en-us/windows/win32/medfound/aac-encoder),
[FFmpeg limiter options](https://ffmpeg.org/ffmpeg-filters.html#alimiter), and
[FFmpeg packet timestamps and durations](https://ffmpeg.org/ffmpeg-bitstream-filters.html#setts).
