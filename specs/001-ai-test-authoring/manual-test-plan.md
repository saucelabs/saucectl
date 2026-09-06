# Manual Test Plan: AI Test Authoring (PR #1100)

**Feature**: [spec.md](./spec.md) | **Quickstart**: [quickstart.md](./quickstart.md) | **Research**: [research.md](./research.md) | **Date**: 2026-09-06

## Context

PR #1100 (`001-ai-test-authoring` → `main`) adds the `saucectl authoring` command group (31 commands over
the AI Authoring API) and a `kind: authoring` runner for `saucectl run`. Automated coverage is strong
(unit tests for client, runner and command helpers; CI green except the pre-existing `xctest` e2e job), and
most paths were exercised once against the live service during development. Manual testing is still
needed for three reasons:

1. **Interactive behaviour** cannot be automated here: confirmation prompts, masked value prompts, the
   export-target picker, the spinner, Ctrl-C mid-run.
2. **Shared surfaces changed**: `run.go` dispatch, the root command list, and the schema bundle that every
   kind validates against. These need a regression check by a human, not just unit tests.
3. **Live-service behaviours** were verified once by one person. A second pair of eyes, on a different
   machine and account, is the point of a manual pass.

Writing this plan surfaced two code follow-ups (see **Known gaps**); they are recorded here so testers do
not report them as new, and they will be fixed on the branch separately.

## Impact analysis: what changed and what it can break

| Change | Files | User-visible impact | Regression risk |
|---|---|---|---|
| New command group | `internal/cmd/authoring/*`, `cmd/saucectl/saucectl.go` | 31 new commands under `saucectl authoring`; root help gains one entry | Root registration only; other groups untouched |
| New run kind | `internal/authoring/*`, `internal/cmd/run/authoring.go`, `internal/cmd/run/run.go` | `kind: authoring` configs run; one new `if` in the dispatch chain | Other kinds dispatch through the same chain; an ordering mistake would misroute |
| Schema | `api/global.schema.json`, `api/v1alpha/framework/authoring.schema.json`, `api/saucectl.schema.json` | `kind` enum gains `authoring`; new branch. **Every kind's advisory validation reads this bundle once merged** | A malformed bundle would print spurious validation errors for cypress/playwright/etc. |
| HTTP client | `internal/http/authoring.go` | New client with its own retry policy | Does not touch the shared client; `NewRetryableClient` unchanged |
| Fixture | `.sauce/authoring.yml` | Runnable sample config | None |
| CI / Makefile | `.github/workflows/test.yml`, `Makefile` | CI binaries now carry a real version; `make schema` uses `cd` | Already observed working in PR CI ("Running version v0.0.0+1788be91") |
| Spec-kit + skills | `.specify/`, `.claude/skills/`, `specs/` | Repository tooling and docs | No runtime impact; out of scope here |

## Preconditions

| Need | Why | Notes |
|---|---|---|
| Binary built from the branch | all scenarios | `git fetch origin 001-ai-test-authoring && git checkout 001-ai-test-authoring && make build` → `./saucectl` |
| Credentials for an org **with** the AI authoring entitlement | everything except A3 | `saucectl configure` or `SAUCE_USERNAME`/`SAUCE_ACCESS_KEY`. The reference org (us-west-1) holds ~186 cases |
| Credentials for an org **without** the entitlement | A3 only | Optional but valuable; skip if unavailable and record "not run" |
| A real terminal (TTY) for stdin and stdout | prompts, spinner, Ctrl-C | Do not run those scenarios under `script`, CI or a pipe |
| A running Sauce Connect tunnel | D4, J16 | Optional; mark "not run" if none |
| Real-device access | J18 | Optional; no authored real-device test case exists today |
| `jq` | JSON assertions | Any recent version |

**Rules of engagement.** Create your own assets and delete them at the end. Name everything
`manual-<initials>-<yyyymmdd>-…` (variables: `manual_<initials>_…`, lowercase and underscores only) so
leftovers are findable. Author tests against `https://www.saucedemo.com` only. Never delete, rename or
re-suite a test case you did not create. Variables at `team` scope unless a row says otherwise. Pass
`--disable-usage-metrics` to keep analytics clean. Until the PR merges, every `saucectl run` with this kind
prints one red advisory line ("value must be one of … in /kind"); that is expected (J19).

**Known service facts to keep in mind** (from research.md): run completion appears on the run resource
~20 s after start; build names come back as `"<name> - 1"`; `UTC` is not a valid schedule timezone;
schedule cron has six fields with seconds first; variables listing needs `--limit` 1–200 while test cases,
suites and schedules accept `--limit 0` as count-only; run history
outlives a deleted test case; code export output differs between calls; a suite's `testCaseCount` in a
create/update response can lag behind the change.

## Known gaps found while writing this plan

These are real and will be fixed on the branch; test them as described and do not file them again.

| Gap | Where | Effect | Row |
|---|---|---|---|
| Dynamic shell completion for `testcases code --target` returns nothing: cobra runs no pre-run hooks during completion, so the service client is never initialised and the completion function bails out | `internal/cmd/authoring/testcases_code.go` | `--target <TAB>` offers no values. Static completions for `--scope` and `--state` are unaffected | H9 |
| Bad credentials read as "the current user has no organisation": the shared user lookup does not check the HTTP status, so a 401 body decodes to an empty user | `internal/http/userservice.go`, `internal/authoring/entitlement.go` | The message is still a "could not verify" error, distinct from "not in your plan", but the reason given is imprecise | A4 |

## Legend

- **Priority**: P1 = must pass before merge; P2 = should pass; P3 = nice to have / optional environment.
- **Coverage**: `unit` = unit-tested; `live` = exercised once against the live service during development;
  `manual-only` = only a person can verify this. Focus effort on `manual-only` and P1.
- Placeholders: `<TC>` = the test case you author in B1; `<TC2>` = the second one from B5; `<SUITE>` from
  E1; `<SUITE2>` = an empty second suite created in F6; `<SUITE3>` = a suite sharing `<SUITE>`'s exact name,
  created and deleted inside J5; `<SCHED>` from F2; `<VAR>`, `<VAR2>` from the G group.
- **Sequencing that matters**: E1 before C1 and B12; F2 before E6; F6 before E10; J5 and J6 before E9
  (which deletes `<SUITE>`); E2 renames `<SUITE>`, so later rows use its *current* name.
- Every command below is `./saucectl authoring --disable-usage-metrics …` unless it starts with `./saucectl run`.

---

## A. Access, entitlement and region

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| A1 | Help needs no network | Disconnect network or set bogus creds, run `authoring --help`, `authoring testcases --help` | Help renders; no entitlement error, no delay | P1 | live |
| A2 | Entitled org proceeds | `testcases list --limit 1` | One row, footer `showing 1 of N test cases` | P1 | live |
| A3 | Not in plan is a distinct message | Use creds of an org without the entitlement: `testcases list`; also `./saucectl run -c <any authoring cfg> --dry-run` | Both say AI Test Authoring is **not included in your plan** and name the account team; no mention of credentials | P1 | unit |
| A4 | Bad credentials is a distinct message | `SAUCE_ACCESS_KEY=wrong testcases list` | Error begins **could not verify AI authoring entitlement** and ends "the current user has no organisation" (the 401 body is `{"detail":"Authorization failed"}`, which the shared user lookup decodes as an empty user — see Known gaps); it must **not** say "not in your plan" | P1 | unit / live (401 body verified) |
| A5 | No credentials | Unset the env vars and move `~/.sauce/credentials.yml` aside; `testcases list` | Error names `saucectl configure` and the two env vars | P2 | manual-only |
| A6 | Region before and after subcommand | `-r eu-central-1 testcases list --limit 1`; `testcases list --limit 1 -r eu-central-1`; `-r us-east-4 …` | All work; totals differ per DC (reference: 186 / 38 / 2) | P2 | live |
| A7 | Invalid region | `-r mars testcases list` | `invalid region "mars"; options: us-west-1, us-east-4, eu-central-1` | P2 | live |

## B. Authoring from the terminal

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| B1 | Author with `--wait` on a TTY | `testcases generate --name "manual-<i>-login" --intent "Open https://www.saucedemo.com, log in with username standard_user and password secret_sauce, and verify the Products heading is visible." --test-url https://www.saucedemo.com --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' --tag manual-<i> --wait` | "Generation task accepted" with Task ID and Sauce job ID; a spinner while queued/in progress; `* <reasoning title>` and `✓ <action>` lines appear, each exactly once, as the agent works; ends with `Generation completed. New test case: <TC>` and the inspect hint; exit 0. Observed duration ≈ 50 s | P1 | unit (rendering) / manual-only (live streaming, spinner) |
| B2 | Interrupt the wait | Repeat B1 with a new name; press Ctrl-C after ~10 s | "Waiting for any in-progress actions to stop…", then `generation is still running on Sauce Labs. Check progress with: … generate-status <task> --wait`, then `Error: … context canceled`; exit 1; a second Ctrl-C is not needed | P1 | live |
| B3 | Reattach | `testcases generate-status <task from B2> --wait` | Streams whatever is still pending, or reports completion at once; prints the new test case ID and inspect hint; exit 0 | P1 | live |
| B4 | Snapshot without waiting | `generate-status <task>` and `generate-status <task> -o json` | One-shot status. COMPLETED shows the new ID and hint and **no steps** (the service returns none at that status); JSON is one object | P2 | live |
| B5 | Fire-and-forget authoring | B1 with a new name and without `--wait` | Prints task ID, Sauce job ID and the `generate-status … --wait` hint; exit 0. Wait for completion via B4 before using the result as `<TC2>` | P2 | unit |
| B6 | JSON while waiting | B1 with a new name, `-o json --wait` | No "accepted" header, no streamed lines; exactly one final JSON object with `taskId`, `status`, `testCaseId` | P2 | unit |
| B7 | Authoring that fails | Intent "Open https://www.saucedemo.com and click the button labelled 'Purple Elephant'", `--max-steps 6 --wait` | Either the task ends FAILED — then `generation failed: <code>: <detail>` and exit 1 — or it completes with a ✗ step and a saved case. Record which; both are acceptable, a hang or a silent exit 0 without a case is not | P2 | manual-only |
| B8 | Local wait shorter than the task | B1 with a new name and `--wait-timeout 10s` | After ~10 s: the still-running message with the reattach hint; exit 1; the task continues (confirm with B4) | P2 | unit |
| B9 | Input validation, no request sent | Omit `--name`; omit intent; give both `--intent` and `--intent-file`; omit target; two `--target`s; `--generation-timeout 30s`; `--max-steps 500`; 21 `--tag`s | Each rejected immediately with a precise message naming the flag and the bound | P2 | unit |
| B10 | Intent from stdin and file | `echo "…" \| … --intent-file -`; `--intent-file intent.txt` | Accepted, surrounding whitespace trimmed; `--intent-file -` on a TTY with nothing piped is refused | P3 | unit |
| B11 | Unknown task | `generate-status 00000000000000000000000000000000` | `(HTTP 404) TEST_CASE_GENERATION_TASK_NOT_FOUND: Test case generation task not found.`, answered as fast as any other request | P2 | live (endpoint) |
| B12 | Author straight into a suite | After E1 and after J5/J6 (an extra member would change their expected counts): B5 with a new name and `--test-suite-id <SUITE>` | The new case's `Suite` column shows `<SUITE>`; `testcases list --test-suite-id <SUITE>` includes it | P2 | manual-only |

## C. Inspecting test cases

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| C1 | Filters (after E1) | `list --search manual`; `--tag manual-<i>`; `--user-id $(get <TC> -o json \| jq -r .creatorUserId)`; `--start-date <today>T00:00:00Z`; `--test-suite-id <SUITE>` | Each narrows correctly. Tags are case-sensitive: `--tag Login` and `--tag login` differ. `--test-suite-id null` lists the unassigned cases (reference org: 119, which with the 67 cases inside suites adds up to the 186 total) | P1 | live |
| C2 | Pagination | `list --limit 2`; `--skip 2 --limit 2`; `--all` | Disjoint pages. `--all` fetches everything; the "large listing" warning fires only above 200 total, so in the reference org (~186) expect **no** warning | P2 | live |
| C3 | Count only | `list --limit 0 -o json` | `{"items":[],"total":N}` | P2 | live |
| C4 | Detail and steps | `get <TC>`; `get <TC> --show-steps`; `get <TC> --revision <id from the Revision row>`; `--revision bogus` | Property table; a **Reasoning:** block with the agent's titled paragraphs; then a step table with one readable line per action, ✔/✖, a 32-hex screenshot ID (never a URL) and a truncated per-step reason. Bogus revision → `test case … has no revision bogus` | P1 | live |
| C5 | Empty stored tunnel shown honestly | `get 6a6b903c0405fb400076b2ba` (a colleague's case that still stores `""`; read-only) | Tunnel row reads `"" (empty; cleared automatically on run)` | P3 | unit |
| C6 | Tags list | `list-tags` | Distinct tags with case preserved (`Login` and `login` both appear) | P2 | live |
| C7 | Run listing is scoped (SC-002) | After D1: `list-runs <TC> -o json \| jq -r '[.items[].testCaseId]\|unique'` | Exactly `["<TC>"]` | P1 | live |
| C8 | Run detail | `get-run <TC> <run-id>` | Property table plus a jobs table whose URL is `https://app.saucelabs.com/tests/<sauceJobId>`; the link opens the job | P1 | live |
| C9 | JSON mirrors text | Any listing or detail with `-o json` | Valid JSON with the same records as text | P2 | live |
| C10 | Empty result | `list --search zzzz-nothing` | Single line `No test cases found (total: 0).`, no table frame | P2 | live |
| C11 | Not found is fast | `time … get 000000000000000000000000` | 404 with `TEST_CASE_NOT_FOUND` and detail; wall time ≈ three requests (two for the entitlement gate, one lookup), well under 5 s | P1 | live |
| C12 | Unknown output format | `list -o yaml` | Rejected before any request | P3 | unit |
| C13 | Rename (FR-016) | `rename <TC> "manual-<i>-renamed"`; `get <TC>`; `rename <TC> "$(printf 'x%.0s' {1..300})"` | Renamed and confirmed; the 300-character name is rejected by the service with `INVALID_BODY` naming `name` (record the exact message) | P1 | live (rename) / manual-only (bound) |
| C14 | Run listing filters and paging | After several runs: `list-runs <TC> --limit 1`; `--skip 1 --limit 1`; `--all`; `--start-date <today>T00:00:00Z`; `--user-id <yours>` | Disjoint pages, correct totals, filters narrow; every row's `testCaseId` is `<TC>` | P2 | live (basic) / manual-only (filters) |
| C15 | Run detail enforces its test case | `get-run <TC2> <a run id of TC>` | 404 `TEST_CASE_RUN_NOT_FOUND`: the detail endpoint, unlike the list, does check the path's test case (research R-004) | P3 | live (research) |

## D. Running a single test case from the CLI

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| D1 | Stored targets | `testcases run <TC> --build manual-<i>` | `Run <id> started for test case <TC> (build "manual-<i> - 1")`, a jobs table with the derived URL, the `get-run` hint; exit 0 immediately. ~30 s later `get-run` shows `passed (1/1)` | P1 | live |
| D2 | Key=value target | `run <TC> --target 'browserName=firefox,platformName="Windows 11",browserVersion=latest'` | Accepted; the job target shows firefox; `get-run` reaches a terminal state (pass or fail — record which, the case was authored on Chrome) | P1 | unit (parsing) / manual-only (service) |
| D3 | JSON target from file | `t.json` = `{"browserName":"chrome","platformName":"Windows 11","sauce:options":{"screenResolution":"1280x1024"}}`; `run <TC> --target-json @t.json` | Accepted; `get-run -o json` shows the nested `sauce:options` in the job target | P2 | unit (parsing) / manual-only (service) |
| D4 | Tunnel by name | With a live tunnel: `run <TC> --tunnel-name <name>`; then with a made-up name | Live name accepted; made-up name → `SC_TUNNEL_NOT_FOUND` with the service's detail | P3 | manual-only |
| D5 | Revision path | `run <TC> --revision <rev id from C4>` | Accepted or a clear service error — this path is described but not declared by the API; record the outcome | P3 | unit (path building) |
| D6 | Impossible target fails visibly | `run <TC> --target browserName=chrome,browserVersion=999,platformName="Windows 11"`; poll `get-run` | Within ~10 s the job is `failed` with error `Unable to start your session.` | P2 | live |

## E. Test suites

Run F2 (create a schedule on `<SUITE>`) before E6 so the delete prompt has a schedule to list. Run J5, J6
and E10 before E9, which deletes `<SUITE>`.

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| E1 | Create with members | `testsuites create --name manual-<i>-suite --tag manual-<i> --test-case <TC> -o json`; `testsuites get <SUITE>`; `testcases list --test-suite-id <SUITE>` | ID returned; detail shows name, tags, team and the listing hint; the listing shows `<TC>` (the `testCaseCount` in the create response may still read 0 — that lag is expected) | P1 | live |
| E2 | Rename and retag | `update <SUITE> --name manual-<i>-suite-2 --tag a --tag b`; `get <SUITE>` | Both applied | P2 | live |
| E3 | Incremental membership | With `<TC2>` complete: `update <SUITE> --add-test-case <TC2>`; `update <SUITE> --remove-test-case <TC>` | Listing shows exactly `<TC2>`; `get <TC>` shows `Suite -` again | P1 | live |
| E4 | Exclusive flags and empty update | `update <SUITE> --test-case <TC> --add-test-case <TC2>`; `update <SUITE>` | Both rejected client-side, no request | P2 | live |
| E5 | Fire-and-forget suite run | `testsuites run <SUITE> --build manual-<i>` | `Queued N run(s) for test suite … under build "manual-<i>…"` (record whether this endpoint also decorates the name with ` - 1`) plus the note that results are not followed here; exit 0. This starts real jobs for every member | P2 | manual-only |
| E6 | **Interactive delete prompt** | TTY: `testsuites delete <SUITE>` → answer **N** | `About to delete test suite "…" (…)`, then: N test case(s) kept and becoming unassigned, and each schedule that triggers the suite (from F2); `Proceed?` defaults to No; N → `Error: aborted`, exit 1, suite still exists | P1 | manual-only |
| E7 | Cascade warning | TTY: `testsuites delete <SUITE> --delete-test-cases` → **N** | The prompt says the N test case(s) **will be DELETED** | P1 | manual-only |
| E8 | Non-interactive refusal | `testsuites delete <SUITE> < /dev/null` | `refusing to proceed without confirmation: not running interactively; re-run with --yes to confirm (test suite …)`; exit 1 | P1 | live |
| E9 | Non-interactive with bypass | After the F group: `testsuites delete <SUITE> --yes` | Deleted; `<TC2>` still exists and shows `Suite -` | P1 | live |
| E10 | Listing filters (after F6) | `testsuites list --id <SUITE> --id <SUITE2>`; `--search manual-<i>`; `--limit 0 -o json`; `schedules list --limit 0 -o json` | Exactly those two; search narrows; both count-only requests return `{"items":[],"total":N}` (suites and schedules accept `limit=0`; only variables do not) | P2 | manual-only (filters) / live (count-only) |

## F. Schedules

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| F1 | Timezone required; UTC rejected | `schedules create --name manual-<i>-sched --cron "0 0 3 1 1 *" --test-suite-id <SUITE> --state disabled` (no `--timezone`); then with `--timezone UTC` | First: client error `--timezone is required …(the service does not accept "UTC")`. Second: service `INVALID_BODY` with `settings.timezone: Value must be a valid IANA timezone.` | P1 | live |
| F2 | Create disabled, far future | Same with `--timezone Europe/Berlin --max-runs 1 --build manual-<i> -o json` | Created, `DISABLED`, `runningUserId` equals your own user ID, `nextRunDate` null | P1 | live |
| F3 | Enable / disable | `enable <SCHED>`; `get <SCHED> -o json`; `disable <SCHED>` | State flips each time and **nothing else changes** (cron, timezone, maxRuns, buildName intact) | P1 | live |
| F4 | Partial update keeps the rest | `update <SCHED> --cron "0 0 4 1 1 *"`; `get -o json` | Only cron changed | P1 | live |
| F5 | Unset | `update <SCHED> --unset maxRuns --unset buildName`; `get -o json`; `update <SCHED> --unset cron` | maxRuns and buildName gone, everything else intact; `cannot unset "cron"; options: …` | P1 | live |
| F6 | Membership | Create `<SUITE2>` first: `testsuites create --name manual-<i>-suite-b -o json` (no members needed). Then `update <SCHED> --add-test-suite-id <SUITE2>`; `--remove-test-suite-id <SUITE2>`; `--remove-test-suite-id <SUITE>` (the last one); `--test-suite-id x --add-test-suite-id y` | Add and remove applied; removing the last suite refused (`must keep at least one test suite`); wholesale + incremental rejected | P2 | unit |
| F7 | Empty update | `update <SCHED>` | `nothing to update` | P2 | live |
| F8 | Listing | `schedules list --test-suite-id <SUITE>`; `--id <SCHED>`; `--search manual-<i>` | Each shows `<SCHED>`; **Next Run** reads `-` while disabled | P2 | live (suite filter) / manual-only (id, search) |
| F9 | **Interactive delete prompt** | TTY: `schedules delete <SCHED>` → **N** | Prompt states `it is DISABLED and runs "<cron>" (<tz>)` and `it triggers N suite(s): …`; N aborts, schedule remains | P1 | manual-only |
| F10 | Non-interactive refusal / bypass | `schedules delete <SCHED> < /dev/null`; then `--yes` | Refuse (exit 1), then `Deleted schedule …` | P1 | live |
| F11 | Observed state guard | Only if you own a schedule in `ERRORED` or `RUNNING`: `update <it> --cron …` | Error: `schedule is currently ERRORED; pass --state ENABLED or --state DISABLED to update it` | P3 | unit |
| F12 | Unsettable state and bad cron | `create … --state running`; `create … --cron "not a cron"` | `invalid --state "running"; options: ENABLED, DISABLED` client-side; the bad cron is rejected by the service with `INVALID_BODY` naming `settings.cron` (record the wording) | P3 | unit (state) / manual-only (cron) |

## G. Variables

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| G1 | Secret from env | `MY_SECRET=abc variables create --scope team --name manual_<i>_secret --secret --value-from-env MY_SECRET -o json` | Created; the JSON has **no** `value` key | P1 | unit |
| G2 | Plain from stdin and file | `echo x \| variables create --scope team --name manual_<i>_stdin --value-from-file -`; `printf 'y\n' > v.txt; … --name manual_<i>_file --value-from-file v.txt`; `get` both | Values are exactly `x` and `y`: one trailing newline stripped | P1 | live |
| G3 | **Masked prompt** | TTY: `variables create --scope team --name manual_<i>_prompt --secret` (no value flag) | Masked `Value:` prompt, typed characters not echoed; created | P1 | manual-only |
| G4 | Plain prompt | Same without `--secret`, name `manual_<i>_prompt2` | Plain `Value:` prompt with visible input | P2 | manual-only |
| G5 | `--value` warning | `create --scope team --name manual_<i>_v --secret --value abc` | Created, preceded by a WRN about shell history and the process list | P2 | live |
| G6 | Secret never displayed | `get <VAR>`; `get <VAR> -o json`; `list --scope team --search manual_<i>` in text and JSON | `<secret>` in text; no `value` key in JSON; plain variables show their value | P1 | live |
| G7 | Conflict detection | `get <VAR> -o json` → note `lastUpdate` T1; `update <VAR> --description one`; `update <VAR> --description two --expected-last-update T1` | Second update refused: `… was changed by someone else since it was read; re-read it with 'saucectl authoring variables get <VAR>' …`; exit 1 | P1 | live |
| G8 | Default read-then-write | `update <VAR> --description three` | Succeeds, prints the new version token | P1 | live |
| G9 | Toggle secrecy | `update <VAR> --secret=false`; `get`; `update <VAR> --secret=true`; `get` | Per the API docs the value moves across and becomes visible, then hidden again. Record the actual behaviour | P2 | manual-only |
| G10 | Scope pairing | `list --scope testSuite`; `create --scope org --test-case-id x --name z --value v`; `list --scope testCase --test-case-id <TC>` | First two rejected client-side naming the missing/forbidden identifier; the third works (possibly empty) | P1 | live |
| G11 | Limit bounds | `list --limit 0`; `--limit 201` | Both rejected: `--limit must be between 1 and 200 for variables; the service has no count-only mode` | P2 | live |
| G12 | Non-interactive without a value; empty update | `create --scope team --name manual_<i>_none < /dev/null`; `update <VAR>` with no flags | Error listing `--value-from-env`, `--value-from-file` and `--value`; `nothing to update: specify --name, --description, --secret or a value source` | P2 | unit |
| G13 | Delete with tokens | `delete <VAR> --yes --expected-last-update 2026-01-01T00:00:00.000Z`; then `delete <VAR> --yes` | Stale token → conflict message, exit 1; fresh default → `Deleted variable …` | P1 | live |
| G14 | **Interactive delete prompt** | TTY: `variables delete <VAR2>` → **N**, then again → **Y** | Prompt states `it is a <secret\|plain> variable at team scope` and that tests referencing `{{team:<name>}}` will lose it; N aborts, Y deletes | P1 | manual-only |
| G15 | Suite- and case-scoped variables | `create --scope testSuite --test-suite-id <SUITE> --name manual_<i>_s --value 1`; `create --scope testCase --test-case-id <TC2> --name manual_<i>_c --value 2`; `list --scope testSuite --test-suite-id <SUITE>`; later, after E9 deletes the suite: `get <suite var>` | Both created and listed under their scope. After the suite is deleted, record whether the suite-scoped variable still exists (unknown; both outcomes acceptable, must not be silent) | P2 | manual-only |
| G16 | Name validation | `create --scope team --name "Bad Name" --value v` | Rejected client-side: lowercase letters, digits and underscores only | P3 | unit |

## H. Export to code

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| H1 | Targets | `testcases list-code-targets <TC>` | The org's targets (reference org: nine, incl. `typescript_playwright`, `java_selenium`) | P2 | live |
| H2 | Stdout redirect | `testcases code <TC> --target typescript_playwright > t.spec.ts` | The file is Playwright source; nothing else was written to stdout | P1 | live |
| H3 | File, refusal, force | `code <TC> --target python_selenium -f test_login.py`; again; again with `--force` | Written; `… already exists; use --force to overwrite`; overwritten | P1 | live |
| H4 | Directory with derived name | `code <TC> --target java_selenium -d out/` | File named after the source's `public class`, e.g. `SauceDemoLoginTest.java` | P1 | live |
| H5 | Unavailable target | `--target cobol_thing` | Error listing the valid targets | P2 | live |
| H6 | **Interactive target picker** | TTY: `code <TC>` (no `--target`) | A select prompt listing the targets; choosing one prints the source | P1 | manual-only |
| H7 | Non-interactive without target | `code <TC> < /dev/null` | `--target is required; available: …` | P2 | live |
| H8 | JSON | `code <TC> --target python_selenium -o json` | `{"target":…,"code":…}` | P3 | live |
| H9 | Shell completion | With completion installed (`saucectl completion <shell>`): `authoring variables list --scope <TAB>`; `authoring schedules create --state <TAB>`; `authoring testcases code <TC> --target <TAB>` | `--scope` offers the four scopes and `--state` offers ENABLED/DISABLED. `--target` currently offers **nothing** — Known gap, record as such, not as a defect | P3 | manual-only |

## I. Artifacts

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| I1 | Download by ID | Take a Screenshot ID from C4: `download-artifact <id> -f step.png`; open the file | `Wrote N bytes to step.png`; a PNG of the step | P1 | unit (client) / live (endpoint, during research) |
| I2 | Download by full URL | `URL=$(get <TC> -o json \| jq -r '.revisions[-1].steps[0].screenshotUrl'); download-artifact "$URL" -f step2.png` (quotes matter: the URL contains `&`) | Same result; the identifier is extracted from the URL | P2 | unit |
| I3 | Destination required; no overwrite | Omit `-f`; repeat with an existing file; add `--force` | `a destination is required`; `already exists; use --force`; overwritten | P2 | unit |
| I4 | Unknown ID | `download-artifact 00000000000000000000000000000000 -f x.bin` | 404 `FILE_NOT_FOUND`, answered in one request | P2 | live (research) |

## J. Pipeline runs with `kind: authoring`

Start from `.sauce/authoring.yml`, point `testCases` at `<TC>` (and `<TC2>`), enable
`reporters.junit` and set `artifacts.download.when: always`.

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| J1 | Dry run | `./saucectl run -c cfg.yml --dry-run` | `The following test cases would have run:` with one line per suite → case (id) stating stored or configured targets; nothing starts (`list-runs <TC>` count unchanged) | P1 | live |
| J2 | Passing run | `./saucectl run -c cfg.yml` | INF `Run started` with the job URL, `Runs in progress: N` every 10 s, `Run finished`, a results table with ✔ rows, `All suites have passed`, a **Build Link** that opens, exit 0 | P1 | live |
| J3 | Failing run | A suite with `targets: [{capabilities: {browserName: chrome, browserVersion: "999", platformName: "Windows 11"}}]` | ERR `Run finished with failures`, ✖ row `failed`, footer `1 of N suites have failed`, exit 1; JUnit `<failure message="Unable to start your session.">`; Build Link `N/A` (the job never existed) | P1 | live |
| J4 | One row per target | One suite with two targets (chrome and firefox) | Two rows for one test case with distinct Browser cells; JUnit has two `<testcase>` elements | P1 | unit |
| J5 | Suite by name | `testSuiteName:` set to `<SUITE>`'s **current** exact name (`manual-<i>-suite-2` after E2); then `manual` (substring); then a name two suites share (create `<SUITE3>` with exactly that same name first; delete it with `--yes` afterwards) | Exact resolves; substring → `no test suite is named "manual" (the match is exact and case-sensitive)`; duplicate → `2 test suites are named …; reference one by testSuiteId instead` | P1 | unit |
| J6 | Suite by ID with tags | Tags can only be set at authoring time (there is no tag-edit command), so author `<TC2>` in B5 with an extra `--tag manual-<i>-only`. E3 leaves only `<TC2>` in the suite, so re-add `<TC>` first: `testsuites update <SUITE> --add-test-case <TC>`. Config: `testSuiteId: <SUITE>` plus `tags: [manual-<i>-only]` | Only `<TC2>` runs; without `tags`, both run | P2 | unit |
| J7 | Concurrency | Three test cases in one suite, `--ccy 1`; watch the `Run started` timestamps | Runs start one after another, each after the previous finishes (~25 s apart) | P2 | unit |
| J8 | Async | `--async` | INF `Run started (async)` with the `get-run` hint, rows `in progress`, footer `All suites have launched`, exit 0; no JUnit/JSON files written | P1 | unit |
| J9 | JUnit has test cases (SC-008) | `grep -c '<testcase' saucectl-authoring-report.xml` after J2 | Equals the number of jobs | P1 | live |
| J10 | JSON reporter | `--reporters.json.enabled` | `saucectl-report.json` with one entry per job | P2 | unit |
| J11 | Artifacts | `when: always`, match `*.mp4`, `log.json`; then `when: fail` on a passing run; then `artifacts.cleanup: true` with an old file in the directory | `./artifacts/<suite_-_case>/video.mp4` and `log.json`; nothing downloaded on `fail`; cleanup empties the directory before the run | P1 | live (download) / unit (skip rules) |
| J12 | Timeout | `defaults.timeout: 5s` | ERR `Timed out waiting; the run may still be going on Sauce Labs. Check it with: …`; the row shows ✖ with status `?`; footer counts it as failed; exit 1 | P1 | unit |
| J13 | **Ctrl-C mid-run** | Start J2; press Ctrl-C after `Run started` | WRN `Interrupted locally; the run continues on Sauce Labs. Check it with: …`; the results table still renders (row `in progress`); exit 1; `get-run` a minute later shows the run finished. Pressing Ctrl-C **before** any `Run started` line yields `Run was not started: interrupted.` instead | P1 | manual-only |
| J14 | Unsupported settings warn | Dry run with `--retries 2 --tags a --env X=1 --launch-order "fail rate" --show-console-log --live-logs --fail-fast` | One WRN per setting naming it as not supported for kind: authoring; the run proceeds | P1 | live |
| J15 | Select suite | `--select-suite "<name>"`; `--select-suite nope` | Only that suite runs; `no suite named 'nope' found` | P2 | unit |
| J16 | Tunnel | `sauce.tunnel.name: <live tunnel>`; then a dead name | Readiness check logs `Tunnel is ready!` and runs; the dead name fails **before** any run starts, after the tunnel timeout | P3 | manual-only |
| J17 | Case with empty stored tunnel | Only with the owner's agreement: run `6a6b903c0405fb400076b2ba` (Android emulator, stores `""`) with no tunnel configured | Starts and completes because the runner sends `scTunnelName: null` (verified 2026-09-05 on another case, whose stored value was cleared by that run) | P3 | live |
| J18 | Real device | Author a case on a real device (needs entitlement; how the service marks a target as real-device is not documented — record what works); run it | Its row lands in the RDC table with its own build link | P3 | unit |
| J19 | Schema advisory | Before merge: any run prints the red `/kind` advisory once. After merge: gone | Matches on each side of the merge | P2 | live |
| J20 | Build name | `--build "manual-<i>-x"`; then a 120-character build name | Jobs grouped under `manual-<i>-x - 1` in the dashboard, Build Link points at it; the long name warns and is truncated to 100 characters | P2 | live (grouping) / unit (truncation) |
| J21 | Another data centre | Author a case with `-r eu-central-1`; config with `sauce.region: eu-central-1` | Runs there; job URL uses `app.eu-central-1.saucelabs.com` | P3 | manual-only |
| J22 | Configuration validation | Configs with: no `suites`; a suite with no name; a suite with both `testSuiteId` and `testCases`; a suite with neither; an empty `testCases` entry; a target without `capabilities`; two suites with the same name; no `sauce.region` | Each rejected before any request with a message naming the suite and the rule | P1 | unit |

## K. Regression on untouched surfaces

| ID | Scenario | Steps | Expected | Pri | Coverage |
|---|---|---|---|---|---|
| K1 | Other kinds still validate | After merge: `./saucectl run -c .sauce/cypress-10.yml --dry-run`; `-c .sauce/playwright.yml --dry-run` | No advisory validation errors that `main` did not already print | P1 | manual-only |
| K2 | Other kinds still dispatch | `./saucectl run -c .sauce/playwright.yml --dry-run` (before or after merge) | Runs the playwright path (bundles and validates), not `unknown framework configuration` | P1 | manual-only |
| K3 | Root help | `./saucectl --help` | `authoring` listed once among the other groups | P2 | live |
| K4 | Unrelated groups unaffected | `./saucectl storage list`; `./saucectl builds list vdc --size 1`; `./saucectl devices list` | Behave as on `main` | P2 | manual-only |
| K5 | Bundle is reproducible | `make schema && git status --short api/` | No diff | P1 | live |
| K6 | CI stamps the version | Open the PR's `build` job log | `Running version v0.0.0+<sha>`, not `0.0.0+unknown` | P2 | live (observed) |
| K7 | `make schema` on macOS | Run `make schema` on a Mac with the default `/bin/sh` | Regenerates without a `pushd` error | P3 | manual-only |
| K8 | Every command documents itself | `for c in $(./saucectl authoring testcases --help \| sed -n '/^Available Commands:/,/^$/p' \| awk '/^  [a-z]/{print $1}'); do ./saucectl authoring testcases $c --help \| grep -q Examples: \|\| echo "missing: $c"; done` (repeat for testsuites, schedules, variables; the `sed` keeps the Aliases line out of the loop) | Nothing printed: every leaf command has an Examples section | P3 | live |
| K9 | Aliases | `authoring tc ls --limit 1`; `ts ls`; `schedule ls`; `var ls --scope team --limit 1`; `tc runs <TC>`; `tc tags`; `artifact --help` | All resolve to their commands | P3 | manual-only |

## L. Confirmation matrix (FR-037 to FR-041)

Run each cell once; the interactive cells are the manual focus. Wording is identical across rows apart
from the asset description.

| Asset | Interactive, no `--yes` | Interactive, `--yes` | Non-interactive, no `--yes` | Non-interactive, `--yes` |
|---|---|---|---|---|
| test case (`<TC2>` at the end) | prompt lists its suite membership if any, that its recorded runs stay in history orphaned if any, and the revision count only when there is more than one; N aborts | deletes with no prompt | refuses, exit 1 | deletes |
| test suite | E6 / E7 | manual | E8 | E9 |
| schedule | F9 | manual | F10 | F10 |
| variable | G14 | manual | `delete <VAR> < /dev/null` | G13 |

---

## Teardown

1. `testcases list --search manual-<i>` → `delete <id> --yes` for each.
2. `testsuites list --search manual-<i>` → `delete <id> --yes` for each (without `--delete-test-cases` unless every member is yours and already handled in step 1).
3. `schedules list --search manual-<i>` → `delete <id> --yes` for each.
4. `variables list --scope team --search manual_<i>` → `delete <id> --yes` for each; also the suite- and case-scoped ones from G15 if they survived.
5. `testcases list-tags` must no longer show `manual-<i>`.
6. Remove local `./artifacts`, report files and exported sources.

## Execution order and effort

| Pass | Rows | Time |
|---|---|---|
| Smoke first | A1, A2, C11, J1, J2, K3, K5 | 30 min |
| P1 core (respect the sequencing notes in the Legend) | B1–B3, C1, C4, C7, C8, C13, D1, D2, E1, E3, F1–F5, F9, F10, J5, E6–E9, G1–G3, G6–G8, G10, G13, G14, H2–H4, H6, I1, J3, J4, J8, J9, J11–J14, J22, K1, K2, L | 4–5 h |
| P2 | remaining P2 rows (J6 and E10 before E9) | 2 h |
| P3 / optional environment | B10, C5, C12, C15, D4, D5, F11, F12, G16, H8, H9, J16–J18, J21, K7–K9 | as available |

## Recording results

For each row record: pass / fail / not run (with reason), the command actually typed, and for failures the
full output plus the run or task ID. Severity guide: **blocker** = wrong exit code, data shown for the wrong
test case, a secret displayed, a delete without confirmation; **major** = a documented behaviour absent or a
misleading message; **minor** = formatting. File blockers against PR #1100 before merge. Do not file the two
Known gaps; they are already tracked.

## Out of scope

- Real-device authoring beyond J18; performance under very large organisations (>1000 cases); the spec-kit
  tooling and skills; the pending sauce-docs documentation.
