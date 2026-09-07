# Full Demo render progress

Full Demo preparation previously stayed at 5% while processing every voice
track and round. Only the final video assembly reported FFmpeg progress, and
mastering and delivery validation had no distinct stages.

The editor now assigns estimated weights to preparation, assembly, mastering,
and validation. FFmpeg media timestamps advance each pass; successful work
advances the phase boundaries. No percentage is based on elapsed time. Phase
labels reach both the Clips card and the activity menu, including changes at
the same percentage. The final 100% is written only after the result is saved.

Validation covers the real FFmpeg sponsor/music/voice canaries, preserved
loudness measurements and failure diagnostics, monotonic progress, failed
result writes, API and batch projections, and activity updates at unchanged
percentages. Media filters, approved timelines, and loudness acceptance rules
are unchanged by this progress fix.
