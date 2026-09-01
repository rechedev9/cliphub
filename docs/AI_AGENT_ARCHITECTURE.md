# AI-agent-oriented system design

This document is a target architecture for making ClipHub easier for AI coding and operator agents to understand, plan, and safely drive **without changing product behavior**. It does not introduce generated media decisions, hosted agents, or model-driven clip selection. The demo and persisted edit plans remain the source of truth.

## Goals

- Keep ClipHub deterministic: agents may propose workflows, inspect artifacts, and explain options, but durable demo/stream facts still decide capture and render inputs.
- Make every stage discoverable by contract rather than prose guesses.
- Let agents resume or retry work from durable artifacts without hidden in-memory context.
- Preserve human gates before paid/cloud work, live CS2/HLAE capture, long FFmpeg renders, thumbnail upload-readiness, credentials, or report-adjacent anticheat actions.
- Keep AI integration local-first and optional. No feature should require a hosted backend or a model call to run the pipeline.

## Non-goals

- No autonomous cheating reports, mass reports, or guilt verdicts.
- No AI deciding kills from rendered pixels.
- No resurrected external MCP server.
- No React inside `desktop/src`, no domain rules in `cmd/`, and no new broad `util`/`manager` packages.
- No second implementation of share-code decoding in TypeScript.

## Design principles

1. **Plans over prompts** — the durable plan (`killplan`, `moments`, `streamclips.EditPlan`, `timelineplan.Document`, `tacticalplan.Document`) is the unit of work. Prompts can help select or explain, but later stages consume versioned plans.
2. **Capabilities over assumptions** — agents should discover support with `zv capabilities`, `zv verify doctor`, `zv flows show`, `zv workflows show`, `zv presets`, and `zv skills list` instead of parsing README examples as contracts.
3. **Artifacts over memory** — every resumable decision should be represented by an artifact key, schema version, provenance, or journal entry. A fresh agent session must be able to inspect state from disk/API.
4. **One owner per boundary** — each package owns either facts, plans, side-lane analysis, execution, or presentation. Cross-boundary shortcuts are technical debt even if they work.
5. **Safe-by-default automation** — dry-run first, explicit argv reuse, human approval for irreversible/expensive work, and redacted credentials by default.
6. **Side lanes stay side lanes** — anticheat, tactical, FACEIT indexing, trends, and voice probes can inform review but must not mutate production job state unless their owning contract says so.

## Agent operating model

```text
Agent intent
  -> discover capabilities/workflow contracts
  -> inspect durable inputs and artifacts
  -> create or validate a versioned plan
  -> present unanswered gates to the human
  -> execute the approved command with explicit flags
  -> inspect journal/QA/output metadata
  -> report facts and next safe actions
```

The agent should never rely on hidden chat context for stage transitions. If a later command needs a choice, path, selected player, effect policy, crop, music source, or cover strategy, that information must be present in a plan artifact, command argv, or recorded approval.

## Target package boundaries

| Layer | Existing owners | Agent-facing design intent |
|---|---|---|
| CLI contract | `cmd/zv` | Stable discovery and dry-run validation surface. Add new automation affordances here before adding another binary. |
| Durable demo facts | `parser`, `rules`, `killplan`, `moments` | Produce versioned evidence and selected moments; reject unknown schema versions. |
| Durable stream/timeline facts | `streamclips`, `timelineplan`, `tacticalplan` | Persist canonical edit/tactical documents; renderers consume documents, not ad hoc flags. |
| Execution | `recording`, `composition`, `editor`, `renderplan`, `timelinerender`, `rhythm` | Execute already-approved plans and emit inspectable config/QA/provenance. |
| Orchestration | `httpapi`, `workers`, `tasks`, `job`, `obs` | Durable job state, one capture lane, stable failure classes, journal-first observability. |
| Artifact filesystem | `artifacts`, `storage`, `filecommit` | Centralize artifact keys and atomic writes so agents can locate and trust outputs. |
| External inputs | `faceit`, `sharecode`, `steamresolve`, `steamgc`, `steamclient`, `vodfetch`, `youtube*` | Keep credentials scoped, redacted, explicit, and separate from deterministic clip evidence. |
| UI clients | `web`, `desktop`, `tuiclient` | Present the same pipeline contracts; never become alternate domain implementations. |

## Contract shape for agent-ready stages

Every stage that agents are expected to drive should expose these properties through CLI JSON, HTTP JSON, or a persisted artifact:

- `schema_version`: versioned contract with explicit unknown-version behavior.
- `input_refs`: artifact keys, source paths, checksums, or IDs used to produce the result.
- `decision_basis`: parsed demo facts, stream plan ranges, human approval ID/text, or deterministic ranking formula.
- `effective_config`: all non-default choices after preset expansion, including negative booleans.
- `safety_gates`: unanswered approval/credential/media gates that block live execution.
- `resume_policy`: what can be skipped on retry and what must use a fresh namespace.
- `qa_status`: pass/warn/fail plus exact intervals or artifact paths for review.
- `provenance`: tool versions, HLAE/CS2/FFmpeg detection, music license evidence, and relevant hashes.

## Recommended refactor seams (behavior-preserving)

These are documentation/design targets for future small PRs. Each can be done without changing pipeline behavior when covered by existing or added tests.

1. **Move command leaks behind internal owners**
   - `cmd/zv-recorder`: move launch/teardown orchestration into `internal/recording` or a narrowly named execution package.
   - `cmd/zv-orchestrator`: move SQLite repositories and inline queue behind `internal/job`/`internal/tasks`/`internal/workers` boundaries.
   - `cmd/zv-demo-players`: reuse parser-owned roster extraction or create a parser-facing roster contract.
   - `cmd/zv-analysis-viewer`: either mark as dev-only or move loopback HTML ownership into an internal package.

2. **Make workflow contracts self-describing**
   - Ensure `flows show` and `workflows show` include required artifacts, blocking gates, dry-run/live behavior, and produced artifact keys.
   - Add tests that README/agent docs reference only supported command/preset names.

3. **Normalize artifact manifests**
   - Align demo, stream, timeline, anticheat, and publish-pack outputs around common fields: schema, input refs, effective config, QA, and provenance.
   - Keep artifact key definitions centralized in `internal/artifacts`.

4. **Strengthen approval records**
   - Persist creative brief answers in a machine-readable form before live capture/render.
   - Store explicit negative choices (`--covers=false`, `--hook=false`, etc.) so future agents cannot re-enable them by relying on defaults.

5. **Expose review-first side lanes**
   - Keep anticheat/tactical/voice/trends outputs as review artifacts with clear limitations and no production status mutation.
   - Add CLI/API responses that say how to inspect evidence rather than implying autonomous action.

6. **Codify retry/resume rules**
   - Keep recording `MaxRetry(0)` for deterministic `demo_incompatible:` failures.
   - Document fresh namespace requirements in the workflow JSON and job journal.

## Agent safety checklist

Before live capture/render or upload-ready delivery, an agent must verify:

- The selected workflow was discovered from `zv` JSON, not guessed.
- The command was validated with `--dry-run --format json` when supported.
- All creative brief choices are answered and represented as explicit command values or plan fields.
- The source demo/stream plan and selected player/moments match the requested narrative.
- CS2/HLAE/FFmpeg/tool versions and constraints are known for the run.
- QA warnings are inspected at their exact intervals or intentionally documented.
- Publish pack MP4, cover, title, caption, hashtags, manifest, and metadata describe the same facts.
- Credentials, Steam secrets, FACEIT keys, and third-party music evidence are never printed or committed.

## Migration approach

Use thin, reviewable changes:

1. Add or update contract tests around the existing behavior.
2. Extract one boundary at a time from `cmd/` into a named internal owner.
3. Preserve CLI/HTTP output shape unless a versioned contract says otherwise.
4. Add provenance fields as optional/read-only first, then require them after all producers emit them.
5. Keep README user-facing; put operational detail in purpose-specific docs and link from agent guides.
