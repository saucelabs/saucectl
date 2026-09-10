# Implementation Plan: AI Test Authoring in saucectl

**Branch**: `001-ai-test-authoring` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-ai-test-authoring/spec.md`

## Summary

Add a `saucectl authoring` command group giving complete coverage of the Sauce Labs AI Test Authoring
service (29 endpoints across test cases, suites, schedules, variables and artifact storage, plus a
non-spec entitlement check), and a new `kind: authoring` configuration so authored suites execute through
`saucectl run` alongside the existing frameworks — inheriting the shared reporters, artifact download,
concurrency control and CI exit codes.

The technical approach is dictated by nine behaviours verified against the live service, several of which
contradict its published specification. These are recorded in [research.md](./research.md) and are the
reason this plan deviates from a straightforward REST-wrapper design in four specific places: basic auth
rather than bearer, client-side suite expansion rather than the server's suite-run endpoint, an explicit
query parameter for run listings whose path parameter is ignored, and synthesized JUnit content because
the runner is not a `CloudRunner` and therefore never populates it.

## Technical Context

**Language/Version**: Go 1.26 (`go.mod`)

**Primary Dependencies**: `spf13/cobra` (commands), `spf13/viper` via the `saucelabs/viper` fork
(config), `hashicorp/go-retryablehttp` (HTTP), `jedib0t/go-pretty/v6` (tables), `AlecAivazis/survey/v2`
(prompts), `rs/zerolog` (logging), `mattn/go-isatty` (TTY detection) — all already direct dependencies.
No new modules required.

**Storage**: None local. All state is remote; credentials resolve from `$SAUCE_USERNAME`/`$SAUCE_ACCESS_KEY`
then `~/.sauce/credentials.yml`, unchanged.

**Testing**: `go test` with table-driven tests and `net/http/httptest`; hand-written fakes. Shared fakes in
`internal/mocks`, package-local fakes in the test file (per Constitution IV).

**Target Platform**: Cross-platform CLI — Linux, macOS, Windows.

**Project Type**: CLI (single Go module, `internal/`-scoped packages).

**Performance Goals**: Concurrency bounded by `sauce.concurrency`. A "not found" answered in one request,
not four (the shared retry policy retries 404s and must be overridden). Listing defaults sized so a page
stays under a few hundred KB — measured at ~13 KB per test case.

**Constraints**: No `utils`/`helpers` package (Constitution III). Schema bundle regenerated, never
hand-edited, and byte-identical to CI's (Constitution V). No unbounded waits (Constitution VIII). New
command surface must match existing precedent (Constitution VI).

**Scale/Scope**: 29 service endpoints + 1 entitlement endpoint; 4 resource groups; ~31 commands; 1 new
configuration kind; 1 new runner. Reference organisation: 187 test cases, 16 suites, 6 schedules.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|---|---|---|
| I. Idiomatic Go, enforced by tooling | **PASS** | No new deps; `goimports` grouping; `errname`-compliant `ErrXxx` sentinels and an `APIError` type. Generics (`List[T]`, `ListAll`) are new to this repo but idiomatic on Go 1.26 — noted for review, not a violation |
| II. Document every declaration | **PASS** | Planned for all declarations including unexported. The four verified-behaviour deviations each carry a comment explaining *why*, per the principle's rationale |
| III. No utils/helpers packages | **PASS** | Shared command code lives in named files (`output.go`, `targets.go`, `valuesource.go`) inside `internal/cmd/authoring`, not a `utils` package |
| IV. Plain `go test`, no new frameworks | **PASS** | Table-driven + `httptest`; hand-written fakes; no new test dependency |
| V. Bundled schema generated, never hand-edited | **PASS** | New framework schema + two edits to `api/global.schema.json`, then `make schema`; comparison copy written outside the repo |
| VI. New surface matches existing surface | **VIOLATION — justified below** | Confirmation-before-delete (FR-037–041) departs from `storage delete`, which acts immediately |
| VII. Verify remote behaviour | **PASS** | Nine behaviours verified against the live service; four contradict the published spec. Unverified claims carry verification steps (R1, R4, R12) |
| VIII. Fail loudly, never silently | **PASS** | Unsupported settings warn (FR-013); errors surface the service's own code and detail; every wait bounded (FR-029); non-interactive removal refuses rather than hangs (FR-040) |

**Gate result: PASS with one justified violation** (recorded in Complexity Tracking). No unresolved
`NEEDS CLARIFICATION` markers remain in the specification.

## Project Structure

### Documentation (this feature)

```text
specs/001-ai-test-authoring/
├── plan.md              # This file
├── research.md          # Phase 0 output — the nine verified behaviours and the decisions they force
├── data-model.md        # Phase 1 output — entities, relationships, validation rules
├── quickstart.md        # Phase 1 output — runnable end-to-end validation
├── contracts/
│   ├── cli-surface.md   # Command contract: every command, argument and flag
│   └── config-kind.md   # Configuration contract: the `kind: authoring` schema
├── checklists/
│   └── requirements.md  # Specification quality checklist (complete)
└── tasks.md             # Phase 2 output — created by /speckit-tasks, NOT by this command
```

### Source Code (repository root)

```text
internal/
├── authoring/                   # Domain types, service interfaces, errors, config, runner
│   ├── authoring.go             # Service interfaces, List[T], ListAll
│   ├── testcase.go              # TestCase, Revision, Step, Tool, RunSettings, options
│   ├── testsuite.go
│   ├── schedule.go
│   ├── variable.go
│   ├── run.go                   # Run, RunJob, SuiteRun
│   ├── errors.go                # APIError + code-keyed sentinels
│   ├── config.go                # kind: authoring — Project, Suite, FromFile/SetDefaults/Validate
│   └── runner.go                # RunProject → ResolveTestCases → runSuites → collectResults
├── http/
│   └── authoring.go             # AuthoringService: 29 methods + entitlement check
├── cmd/
│   ├── authoring/               # The `saucectl authoring` command group
│   │   ├── cmd.go               # Root group: region, credentials, entitlement gate
│   │   ├── output.go            # Output format constants, renderJSON, humanizeDate, isTerm
│   │   ├── targets.go           # --target / --target-json capability parsing
│   │   ├── valuesource.go       # --value / --value-from-env / --value-from-file / prompt
│   │   ├── confirm.go           # Confirmation before destructive actions (FR-037–041)
│   │   ├── testcases*.go        # 12 commands
│   │   ├── testsuites*.go       # 6 commands
│   │   ├── schedules*.go        # 7 commands (5 endpoints + enable/disable)
│   │   ├── variables*.go        # 5 commands
│   │   └── artifact.go          # download-artifact
│   └── run/
│       └── authoring.go         # Wiring for `saucectl run` with kind: authoring
└── mocks/
    └── authoring.go             # Shared fakes for command-level tests

api/
├── global.schema.json                      # + "authoring" in the kind enum, + one if/then branch
├── v1alpha/framework/authoring.schema.json # NEW
└── saucectl.schema.json                    # REGENERATED via `make schema`

cmd/saucectl/saucectl.go         # + one import, + one AddCommand entry
.sauce/authoring.yml             # NEW — committed fixture, matching the per-kind convention
```

**Structure Decision**: Single Go module with `internal/`-scoped packages, following the layout every
existing saucectl feature uses. Domain types and their service interfaces live together in
`internal/authoring` (mirroring `build.Service` in `internal/build/build.go`); the HTTP implementation
lives in `internal/http`, satisfying those interfaces. Commands live in `internal/cmd/authoring` as a
single package holding all four subgroups, so the service variables stay unexported and shared — the same
choice `internal/cmd/apit` makes by keeping `vault` in-package. Filenames are prefixed by resource
because four subgroups would otherwise collide on `list.go`.

## Post-Design Constitution Re-Check

*Re-evaluated after Phase 1. Design artifacts: [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md).*

| Principle | Post-design status | What the design added |
|---|---|---|
| I. Idiomatic Go | **PASS** | No dependency was added during design. Generics remain the only new construct, still flagged |
| II. Document every declaration | **PASS** | Each of the four deviations now has a recorded *why* in research.md that becomes the code comment |
| III. No utils/helpers | **PASS** | The file layout in Project Structure holds; `confirm.go` was added as a named file, not a grab-bag |
| IV. Plain `go test` | **PASS** | quickstart.md adds no tooling; the two invisible defects (run filtering, empty JUnit) are covered by assertions rather than manual steps |
| V. Schema generated | **PASS** | contracts/config-kind.md pins the regeneration procedure, including writing the comparison copy outside the repository |
| VI. Match existing surface | **VIOLATION — unchanged, justified** | contracts/cli-surface.md keeps the departure narrow: identical across all four asset types, explicit bypass, and refusal rather than a hang when non-interactive |
| VII. Verify remote behaviour | **PASS — strengthened** | Design surfaced two further verified behaviours (the undocumented `error.data[]` field, and artifact identifiers needing extraction from a pre-signed URL) and demoted one inference to Open-3 rather than asserting it |
| VIII. Fail loudly | **PASS — strengthened** | Design added the non-interactive refusal path (FR-040) and moved the unsupported-setting warnings into `Validate`, so they fire for flag-sourced values too |

**Gate result: PASS**, with the same single justified violation. No new violations were introduced by the
design phase, and no complexity was added that is not recorded below.

One honest note for reviewers: three behaviours (Open-1, Open-2, Open-3 in research.md) remain
**unverified inferences** rather than observations. Under Principle VII they are labelled as such and each
carries a verification step scheduled before the code that depends on it. They do not block Phase 1
design, but Open-1 and Open-3 must be answered before the runner is written.

## Complexity Tracking

> Filled because Constitution Check records one violation.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Confirmation before destructive actions** (FR-037–041), departing from Principle VI's "match existing surface" and from `storage delete`, which deletes immediately with no prompt | These are **shared organisation-level assets**. Deleting a test suite or schedule destroys state colleagues depend on, with no undo. `storage delete` removes an individually-owned uploaded file, a materially lower-consequence action | Matching `storage delete` exactly was the user's explicit alternative and was rejected on the risk asymmetry. Mitigations keep the departure predictable: an explicit bypass preserves scriptability (FR-039), the behaviour is identical across all four asset types (FR-041), and non-interactive use without a bypass refuses rather than silently prompting or proceeding (FR-040) |
| **Generics** (`List[T]`, `ListAll`, `envelope[T]`) — first use in this repository | Five list endpoints share one envelope shape and one pagination protocol. Generics express that once | Five near-identical list structs plus a duplicated `--all` loop per command was the alternative; rejected as ~120 lines of duplication that must stay in sync. Not a Constitution violation (Go 1.26 idiom), but flagged so reviewers are not surprised by a new construct |
