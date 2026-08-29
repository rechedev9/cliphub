# Architecture backlog: AI-agent-ready ClipHub

Behavior-preserving work only unless a task explicitly says otherwise.

## P0 — contracts and safety

- [x] Extend `zv flows show` / `zv workflows show` JSON with required artifacts, produced artifact keys, blocking gates, dry-run/live behavior, and retry/resume notes.
- [ ] Add contract tests that agent docs reference only supported workflow, flow, preset, and skill names.
- [ ] Persist creative brief approvals in machine-readable form, including explicit negative choices.
- [ ] Normalize publish-pack verification so MP4, cover, title, caption, hashtags, manifest, and metadata can be checked from one artifact graph.

## P1 — package boundaries

- [ ] Extract `zv-recorder` launch/teardown logic into a narrow `internal/recording` owner without changing flags.
- [ ] Move `zv-orchestrator` SQLite repository/inline queue ownership behind internal job/task/worker boundaries.
- [ ] Replace `zv-demo-players` direct demoinfocs roster parsing with a parser-owned roster contract.
- [ ] Decide whether `zv-analysis-viewer` is a dev-only binary or should move loopback HTML ownership into `internal/`.

## P2 — artifact/provenance consistency

- [ ] Standardize stage manifests around schema version, input refs, decision basis, effective config, safety gates, resume policy, QA status, and provenance.
- [ ] Keep artifact key definitions centralized in `internal/artifacts` and make new keys visible to CLI/API inspection.
- [ ] Add provenance fields for tool detection, music licensing evidence, and relevant hashes where missing.

## P3 — review-first side lanes

- [ ] Ensure anticheat, tactical, voice, FACEIT, and trend outputs describe limitations and next review action in JSON.
- [ ] Verify side-lane jobs cannot mutate production demo job status.
- [ ] Add docs/examples that show agents how to inspect side-lane evidence without implying autonomous reports or clip decisions.
