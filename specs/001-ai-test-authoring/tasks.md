---
description: "Task list for AI Test Authoring in saucectl"
---

# Tasks: AI Test Authoring in saucectl

**Input**: Design documents from `/specs/001-ai-test-authoring/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: Test tasks are included. Not because TDD was requested, but because
`.specify/memory/constitution.md` requires it as governance: Principle IV mandates plain `go test`
coverage, and the Development Workflow section states unit tests accompany every change. Tests are
written alongside their implementation, not necessarily before it.

**Organization**: Tasks are grouped by user story so each can be implemented, tested and delivered
independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel — different files, no dependency on incomplete work
- **[Story]**: `[US1]`–`[US4]`, mapping to the prioritized user stories in spec.md
- Every task names an exact file path

## Path Conventions

Single Go module. Domain types and their service interfaces in `internal/authoring/`; HTTP
implementation in `internal/http/`; commands in `internal/cmd/authoring/` and `internal/cmd/run/`;
schema sources in `api/`. See **Project Structure** in [plan.md](./plan.md).

---

## ✅ Former blocking constraint — resolved 2026-09-05

Open-1, Open-2 and Open-3 in [research.md](./research.md) were answered by three real runs of test case
`6a88…` in `us-west-1`:

| Open question | Answer | Effect on tasks |
|---|---|---|
| **Open-1** — is completion reported on the run resource? | **Yes**, `success` appears ~19 s in, *before* the Sauce job completes | T030, T031, T034 keep their planned shape; poll the run resource, 5 s interval |
| **Open-3** — what does an empty stored `scTunnelName` do? | **Rejects the run** with `400 SC_TUNNEL_NOT_FOUND` unless `"scTunnelName": null` is sent | T009, T023, T029: send explicit `null` whenever no tunnel is configured |
| **Open-2** — do authored runs publish standard job assets? | **Yes** — video, logs, screenshots; **no** junit.xml | T033 synthesis is the only JUnit source; T034 download is supported |

The ⚠️ markers below are retained as history; none of those tasks is provisional any more.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the toolchain and lay down package skeletons. This is an existing repository, so
there is no project initialization.

- [x] T001 Verify toolchain prerequisites listed in specs/001-ai-test-authoring/quickstart.md — Go 1.26, golangci-lint **v2** (a v1 binary cannot parse .golangci.yml), and Node 20 with `npm ci` in scripts/json-schema-bundler/
- [x] T002 [P] Create package skeleton with package doc comment in internal/authoring/authoring.go
- [x] T003 [P] Create package skeleton with package doc comment in internal/cmd/authoring/cmd.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: The domain package, the HTTP client and the shared command scaffolding. Everything below
this line is required by at least three of the four user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Domain types

- [x] T004 [P] Define APIError, APIErrorItem, Error(), Is() and the code-keyed sentinels in internal/authoring/errors.go — capture the undocumented `error.data[]` array (research R-007), name every sentinel `ErrXxx` for the errname linter
- [x] T005 [P] Define TestCase, Revision, Reasoning, Step, StepResult, Tool, ToolType, Selector, Target and RunSettings in internal/authoring/testcase.go — `Tool.Args` is json.RawMessage with `Summary()` and `Reasoning()` accessors (data-model.md)
- [x] T006 [P] Define TestSuite, CreateTestSuiteOptions and UpdateTestSuiteOptions in internal/authoring/testsuite.go — pointer fields for PATCH tri-state
- [x] T007 [P] Define TestSchedule, ScheduleSettings, ScheduleState, ScheduleStateName constants and update options in internal/authoring/schedule.go
- [x] T008 [P] Define Variable, VariableScope constants, AllVariableScopes, CreateVariableOptions and UpdateVariableOptions in internal/authoring/variable.go
- [x] T009 [P] Define Run, RunJob, SuiteRun with Done()/Passed()/Status(), and RunOptions with a hand-written MarshalJSON that can emit an explicit null tunnel, in internal/authoring/run.go — `Success` is `*bool`; nil means not yet reported (research R-005, R-008)
- [x] T010 Define List[T], ListAll, maxListPages and the six service interfaces (TestCaseService, TestSuiteService, ScheduleService, VariableService, ArtifactService, EntitlementReader) in internal/authoring/authoring.go — depends on T004–T009
- [x] T011 [P] Test Tool.Summary() and Reasoning() across all twelve tool types plus an unknown type, malformed args, nil args and the recursive in_shadow_root, in internal/authoring/tool_test.go
- [x] T012 [P] Test ListAll pagination edges — exact multiple, partial last page, zero total, a server that never signals the end — in internal/authoring/list_test.go

### HTTP client

- [x] T013 Implement AuthoringService, NewAuthoringService, newAuthoringHTTPClient (overriding CheckRetry so 404s are **not** retried), the doAuthoringJSON envelope helper, newAuthoringError and the query-params helper in internal/http/authoring.go — gate `limit` on whether the user set it, not on it being positive (research R-010)
- [x] T014 Implement all 29 service methods in internal/http/authoring.go — depends on T013. `ListRuns` **must** send `testCaseId` as a query parameter (research R-004); `DownloadArtifact` streams and does not use the JSON helper
- [x] T015 Implement IsAIAuthoringEnabled against `{APIBaseURL}/v2/entitlements/entities/org/{orgID}` in internal/http/authoring.go — decode `value` permissively into `any`; this path is **not** under authoringBasePath (research R-009)
- [x] T016 Write client tests in internal/http/authoring_test.go — httptest, `client.Client.RetryWaitMax = 1 * time.Millisecond`; assert basic auth, envelope decode, 204, error codes, 412, the `testCaseId` query parameter, `expectedLastUpdate` in body vs query, and a handler-invocation count of exactly 1 on 404

### Shared command scaffolding

- [x] T017 [P] Implement output helpers — JSONOutput/TextOutput constants, renderJSON, humanizeDate, isTerm, truncate — in internal/cmd/authoring/output.go
- [x] T018 [P] Implement the confirmation helper in internal/cmd/authoring/confirm.go — prompts naming what is affected, honours a bypass flag, and **refuses** when non-interactive without a bypass (FR-037–041)
- [x] T019 [P] Implement `--target` / `--target-json` capability parsing in internal/cmd/authoring/targets.go — shared by testcases run and generate
- [x] T020 Implement the root command group in internal/cmd/authoring/cmd.go — region flag, credentials, service wiring, and the fail-closed entitlement gate that distinguishes "not in your plan" from "could not verify"; skip the gate for help and completion paths. Depends on T010, T015, T017
- [x] T021 Register the group in the AddCommand block of cmd/saucectl/saucectl.go
- [x] T022 [P] Write shared fakes for the six service interfaces in internal/mocks/authoring.go

**Checkpoint**: Foundation ready — user stories can now begin in parallel.

---

## Phase 3: User Story 1 - Gate a pipeline on AI-authored tests (Priority: P1) 🎯 MVP

**Goal**: A pipeline runs authored suites, reports each outcome in saucectl's standard format, and fails
the build when any test fails.

**Independent Test**: Configure one suite, run it, confirm per-target results are printed and the exit
status is non-zero on failure and zero on success. Delivers a working release gate on its own.

**Open-1 and Open-3 were resolved on 2026-09-05; nothing below is provisional any more.**

### Configuration and schema

- [x] T023 [US1] Implement Project, Suite, Target, FromFile, SetDefaults, Validate and FilterSuites in internal/authoring/config.go — truncate build name to 100 characters, map tunnel name, warn on retries/tags/owner, and normalise nested YAML maps for JSON marshalling (contracts/config-kind.md)
- [x] T024 [P] [US1] Test config in internal/authoring/config_test.go — defaults, validation, FilterSuites, and a FromFile fixture with `capabilities: {browserName: chrome, sauce:options: {name: x}}` locking in the viper fork's key-case preservation
- [x] T025 [P] [US1] Create the framework schema in api/v1alpha/framework/authoring.schema.json — `oneOf` over testSuiteId / testSuiteName / testCases; free-form capabilities, so no platform enum to duplicate
- [x] T026 [US1] Add "authoring" to the kind enum and append the if/then branch in api/global.schema.json
- [x] T027 [US1] Regenerate the bundle with `make schema` and commit api/saucectl.schema.json alongside the sources — write any comparison copy outside the repository
- [x] T028 [P] [US1] Create the committed fixture .sauce/authoring.yml, matching the per-kind convention

### Runner

- [x] T029 [US1] Implement Runner, RunProject and ResolveTestCases in internal/authoring/runner.go — resolve suite name by exact client-side match (service search is a substring match), expand suites via testSuiteId, error on ambiguity
- [x] T030 [US1] ⚠️ Implement the bounded worker pool and runTask in internal/authoring/runner.go — semaphore sized from sauce.concurrency; do **not** copy apitest's unbounded fan-out or its package-level poll variable
- [x] T031 [US1] ⚠️ Implement pollRun in internal/authoring/runner.go — poll before the first tick, short-circuit on an already-terminal start response, treat transient failures as non-fatal, take the suite timeout as an argument rather than a global
- [x] T032 [US1] ⚠️ Implement collectResults and toTestResults in internal/authoring/runner.go — one result per job; set RDC (routes the results table), TimedOut (or a timeout reads as merely unfinished), BuildURL via the build service, and URL derived from sauceJobId
- [x] T033 [US1] Implement JUnit synthesis in internal/authoring/runner.go — build Attempt.TestSuites from the run's jobs when a JUnit reporter is active, or reports contain empty containers (research R-011, FR-006)
- [x] T034 [US1] ⚠️ Implement artifact download via saucecloud.JobService.DownloadArtifacts in internal/authoring/runner.go — populate Status, Passed and TimedOut so skipDownload evaluates correctly
- [x] T035 [P] [US1] ⚠️ Test the runner in internal/authoring/runner_test.go — the two load-bearing cases are a synchronous response (assert polling never happens) and a job with no `success` key decoding to nil; plus multi-target, timeout sets TimedOut, start failure, async, a concurrency ceiling, context cancellation, and JUnit synthesis
- [x] T036 [US1] Implement runAuthoring in internal/cmd/run/authoring.go — createReporters (not a bare table reporter), cleanupArtifacts, tunnel validation with the no-op filter, usage tracking
- [x] T037 [US1] Add the import, the authoringTimeout var and the dispatch branch after the apitest case in internal/cmd/run/run.go

**Checkpoint**: A pipeline can gate on AI-authored tests. MVP complete.

---

## Phase 4: User Story 2 - Manage authored tests from the command line (Priority: P2)

**Goal**: Inspect, organise and maintain authored tests, suites, schedules and shared values without a
browser.

**Independent Test**: List the organisation's authored tests, open one, and confirm its recorded steps
are readable. Valuable as an inspection tool even with nothing else built.

### Test cases

- [x] T038 [US2] Implement the testcases subgroup parent in internal/cmd/authoring/testcases.go
- [x] T039 [P] [US2] Implement `list` in internal/cmd/authoring/testcases_list.go — filters, --skip/--limit/--all, warn when --all and total exceeds 200
- [x] T040 [P] [US2] Implement `get` with --show-steps in internal/cmd/authoring/testcases_get.go — render the **extracted** artifact identifier, never the ~1 KB signed URL
- [x] T041 [P] [US2] Implement `delete` in internal/cmd/authoring/testcases_delete.go — routed through the confirmation helper
- [x] T042 [P] [US2] Implement `rename` in internal/cmd/authoring/testcases_rename.go
- [x] T043 [P] [US2] Implement `run` with --target/--target-json/--build/--tunnel-name/--revision in internal/cmd/authoring/testcases_run.go
- [x] T044 [P] [US2] Implement `list-runs` and `get-run` in internal/cmd/authoring/testcases_runs.go — list **must** pass testCaseId as a query parameter; get-run uses the run's own testCaseId
- [x] T045 [P] [US2] Implement `list-tags` in internal/cmd/authoring/testcases_tags.go — data is a plain []string; tags are case-sensitive, do not fold or deduplicate

### Test suites

- [x] T046 [US2] Implement the testsuites subgroup parent in internal/cmd/authoring/testsuites.go
- [x] T047 [P] [US2] Implement `list` and `get` in internal/cmd/authoring/testsuites_list.go
- [x] T048 [P] [US2] Implement `create` in internal/cmd/authoring/testsuites_create.go
- [x] T049 [P] [US2] Implement `update` in internal/cmd/authoring/testsuites_update.go — require at least one flag; reject --test-case combined with --add/--remove-test-case
- [x] T050 [P] [US2] Implement `delete` and `run` in internal/cmd/authoring/testsuites_delete.go — delete confirms; run is fire-and-forget by contract

### Schedules

- [x] T051 [US2] Implement the schedules subgroup parent in internal/cmd/authoring/schedules.go
- [x] T052 [P] [US2] Implement `list` and `get` in internal/cmd/authoring/schedules_list.go
- [x] T053 [P] [US2] Implement `create` in internal/cmd/authoring/schedules_create.go — default --running-user-id to the caller's own identifier, --timezone to UTC
- [x] T054 [P] [US2] Implement `update` with --unset in internal/cmd/authoring/schedules_update.go — read-modify-write, sending a complete settings object (research Open-4)
- [x] T055 [P] [US2] Implement `enable`, `disable` and `delete` in internal/cmd/authoring/schedules_state.go

### Variables

- [x] T056 [US2] Implement the variables subgroup parent in internal/cmd/authoring/variables.go
- [x] T057 [US2] Implement value sourcing in internal/cmd/authoring/valuesource.go — --value-from-env, --value-from-file (with `-` for stdin, refused on a TTY), masked prompt, exactly-one-source validation, single trailing newline stripped
- [x] T058 [P] [US2] Implement `list` and `get` in internal/cmd/authoring/variables_list.go — validate the scope/identifier pairing client-side for **listing**, not only creation; blank secret values before rendering; show lastUpdate raw
- [x] T059 [P] [US2] Implement `create` in internal/cmd/authoring/variables_create.go — warn when --value is combined with --secret
- [x] T060 [P] [US2] Implement `update` in internal/cmd/authoring/variables_update.go — read-then-write by default, --expected-last-update for strict callers, tri-state --secret via Changed(); surface 412 as a named conflict
- [x] T061 [P] [US2] Implement `delete` in internal/cmd/authoring/variables_delete.go — expectedLastUpdate goes in the **query string** here

### Artifacts and tests

- [x] T062 [P] [US2] Implement `download-artifact` in internal/cmd/authoring/artifact.go — require -f since the response carries no content type
- [x] T063 [P] [US2] Test the pure helpers in internal/cmd/authoring/helpers_test.go — target parsing, value sourcing, the schedule update overlay, and the testsuites mutual-exclusion rule

**Checkpoint**: Full management surface available; User Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Author a test from a description (Priority: P3)

**Goal**: Describe a journey in plain language and watch the AI perform it, without leaving the terminal.

**Independent Test**: Describe a simple journey against a public site, watch actions stream, confirm a
runnable test is saved.

- [x] T064 [US3] Implement the shared polling worker in internal/cmd/authoring/testcases_generate.go — poll first then wait; spinner only on a TTY, identical step lines either way; suppress rendering under -o json and emit one final object
- [x] T065 [US3] Implement `generate` in internal/cmd/authoring/testcases_generate.go — the three distinct timeouts (per-request, --generation-timeout sent to the service, --wait-timeout applied to the command context), --intent/--intent-file mutually exclusive
- [x] T066 [P] [US3] Implement `generate-status` in internal/cmd/authoring/testcases_generate_status.go — reuses the polling worker
- [x] T067 [US3] Implement interruption and timeout handling in internal/cmd/authoring/testcases_generate.go — never os.Exit; print the reattach hint naming generate-status with the task identifier
- [x] T068 [P] [US3] Test the generation polling worker in internal/cmd/authoring/testcases_generate_test.go — drive QUEUED → IN_PROGRESS(2) → IN_PROGRESS(4) → COMPLETED and assert each step renders exactly once; plus a cancelled context

**Checkpoint**: Authoring from the terminal works end to end.

---

## Phase 6: User Story 4 - Take an authored test to code (Priority: P4)

**Goal**: Export an authored test to the team's own language and framework, for review and version
control.

**Independent Test**: Export one test to a chosen target, save it, confirm the file is valid source a
developer would recognise.

- [x] T069 [P] [US4] Implement `list-code-targets` in internal/cmd/authoring/testcases_codetargets.go
- [x] T070 [US4] Implement `code` in internal/cmd/authoring/testcases_code.go — resolve available targets first for a better error than 404 CODE_GENERATION_TARGET_NOT_FOUND; prompt on a TTY when --target is absent, else list valid choices
- [x] T071 [US4] Implement filename derivation in internal/cmd/authoring/testcases_code.go — derive from the target's language prefix, not a fixed table; for java_*, extract the public class name from the generated source or the file will not compile
- [x] T072 [US4] Implement output destinations in internal/cmd/authoring/testcases_code.go — stdout by default so redirection works, -f/--filename and -d/--target-dir for files, refuse to overwrite without --force
- [x] T073 [US4] Register shell completion for --target in internal/cmd/authoring/testcases_code.go
- [x] T074 [P] [US4] Test filename derivation in internal/cmd/authoring/testcases_code_test.go — one case per language prefix, an unknown prefix, java class extraction and its fallback, pathological names

**Checkpoint**: All four user stories independently functional.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [x] T075 [P] Add doc comments to every declaration including unexported ones across internal/authoring/ and internal/cmd/authoring/ — Constitution Principle II is stricter than what revive enforces
- [x] T076 [P] Add `Example:` strings to every command in internal/cmd/authoring/
- [x] T077 [P] Register shell completion for --scope and --state in internal/cmd/authoring/
- [~] T078 (2026-09-05/06: sections 1–7, 8 (dry run, one passing run, one failing run, JUnit, artifact download) and 9 verified against the live service, including authoring with interrupt + reattach and the delete confirmation row for all four asset types. Remaining: 8b — no real-device test case exists in the organisation; and the interactive prompt row, unit-tested only) Run the full quickstart in specs/001-ai-test-authoring/quickstart.md, including section 2a (the run-filtering trap) and 8a (JUnit must contain test cases)
- [x] T079 Run the four CI gates — `make lint`, `make test`, `make build`, and `make schema` with the bundle diff reviewed
- [x] T080 Update specs/001-ai-test-authoring/research.md to record the answers to Open-1, Open-2 and Open-3 as observations, replacing their inference status — done 2026-09-05
- [ ] T081 Draft the documentation pull request against the saucelabs/sauce-docs repository — this repository's README documents no individual commands

---

## Dependencies & Execution Order

### Phase dependencies

- **Setup (Phase 1)**: no dependencies
- **Foundational (Phase 2)**: depends on Setup — **blocks every user story**
- **User Story 1 (Phase 3)**: depends on Foundational, plus **Open-1 and Open-3** for the ⚠️ tasks
- **User Stories 2–4 (Phases 4–6)**: depend on Foundational only; independent of one another and of US1
- **Polish (Phase 7)**: depends on the desired stories being complete

### Within Foundational

```text
T004..T009 (parallel, one file each)
      └──> T010 (interfaces, needs the types)
                └──> T013 (client core) ──> T014 (methods) ──> T016 (tests)
                                        └──> T015 (entitlement)
T017, T018, T019, T022 (parallel)
      └──> T020 (root group, needs T010 + T015 + T017)
                └──> T021 (registration)
```

### Within each user story

Domain and configuration before services; services before commands; commands before wiring. Schema
sources (T025, T026) before regeneration (T027).

### Parallel opportunities

- **Phase 2**: T004–T009 are six independent files; T011/T012 pair with them; T017, T018, T019 and T022 are independent
- **Phase 4**: every `[P]` command file is independent once the subgroup parent exists — T039–T045 after T038, T047–T050 after T046, T052–T055 after T051, T058–T061 after T056 and T057
- **Across stories**: once Phase 2 completes, US2, US3 and US4 can be staffed in parallel; US1 can join once the open questions are answered

---

## Parallel Example: Foundational domain types

```bash
# Six independent files, no shared symbols yet:
Task: "Define APIError and sentinels in internal/authoring/errors.go"
Task: "Define TestCase and Tool in internal/authoring/testcase.go"
Task: "Define TestSuite in internal/authoring/testsuite.go"
Task: "Define TestSchedule in internal/authoring/schedule.go"
Task: "Define Variable in internal/authoring/variable.go"
Task: "Define Run, RunJob and RunOptions in internal/authoring/run.go"
```

## Parallel Example: User Story 2 test case commands

```bash
# After T038 creates the subgroup parent:
Task: "Implement list in internal/cmd/authoring/testcases_list.go"
Task: "Implement get in internal/cmd/authoring/testcases_get.go"
Task: "Implement delete in internal/cmd/authoring/testcases_delete.go"
Task: "Implement rename in internal/cmd/authoring/testcases_rename.go"
Task: "Implement run in internal/cmd/authoring/testcases_run.go"
Task: "Implement list-runs and get-run in internal/cmd/authoring/testcases_runs.go"
Task: "Implement list-tags in internal/cmd/authoring/testcases_tags.go"
```

---

## Implementation Strategy

### MVP: User Story 1 only

1. Phase 1 Setup
2. Phase 2 Foundational — blocks everything
3. **Answer Open-1 and Open-3 with one real run** before starting the ⚠️ tasks
4. Phase 3 User Story 1
5. **Stop and validate**: quickstart section 8, including 8a
6. Ship — a pipeline can now gate on AI-authored tests

### Recommended alternative if the open questions cannot be answered immediately

Build **User Story 2 first**. It is the largest phase, depends on nothing unresolved, and delivers a
usable inspection and management tool on its own. Return to User Story 1 once a real run is possible.
This inverts the priority order deliberately and is the better sequencing when the runner is blocked.

### Incremental delivery

Foundational → US2 (management) → US1 (the gate) → US3 (authoring) → US4 (export). Each increment ships
independently without breaking the previous.

### Parallel team strategy

After Phase 2: one developer per story. US2 is large enough to split by resource — test cases, suites,
schedules and variables are four independent file sets behind their subgroup parents.

---

## Notes

- `[P]` means a different file with no dependency on incomplete work
- ⚠️ marks tasks whose shape depends on Open-1 or Open-3; do not start them on inference
- Commit per task or per logical group; stop at any checkpoint to validate a story independently
- Stage deliberately with `git add <path>` — `.claude/`, `.specify/` and `specs/` are untracked on purpose
- Two defects are invisible to manual testing and are covered by assertions instead: the run-listing
  filter (T016, T044) and empty JUnit output (T033, T035)
