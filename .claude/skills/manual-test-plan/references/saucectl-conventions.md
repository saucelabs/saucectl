# saucectl conventions a manual test plan must respect

Each item states the fact and why the plan has to get it right. Facts marked *observed* were established
against the live service on the date given; when in doubt, re-probe read-only rather than trust this file.

## Building and running

- Build with `make build` → `./saucectl`. A local build prints `saucectl version 0.0.0+unknown` by design;
  telling the tester otherwise sends them chasing a non-bug. CI builds are stamped `v0.0.0+<sha>` since
  commit 50b4399 (before it, CI pointed `-X` at a non-existent package and printed `unknown` too).
- Credentials come from `saucectl configure` (`~/.sauce/credentials.yml`) or `SAUCE_USERNAME` /
  `SAUCE_ACCESS_KEY`. Even `--dry-run` needs them: the credential check runs before the dry-run branch.
- Every test command carries `--disable-usage-metrics` so the analytics stream is not polluted by QA.
- When a probe or a scenario needs `curl -u`, read the key into a shell variable
  (`K=$(sed -nE 's/^accessKey: *//p' ~/.sauce/credentials.yml)`) and never echo it or paste it into a
  document.
- Error output differs by entry point, and the plan must quote the right form:
  - `saucectl run …` reports a returned error as a zerolog line
    `ERR failed to execute run command error="…"` and exits 1.
  - Every other command prints cobra's `Error: …` and exits 1.
  - Warnings are `WRN …` lines; an unrelated `WRN A new version of saucectl is available` may appear and
    must not be counted.
- Configuration validation is **advisory** and reads the schema bundle published from `main`
  (`config.ValidateSchema`). A new `kind` therefore prints a red "value must be one of … in /kind" block
  until the PR merges, and the run proceeds anyway. Plans say so and include the before/after-merge check.

## Schema changes

- `api/saucectl.schema.json` is generated from `api/global.schema.json` and the per-framework schemas;
  never hand-edited. Any plan touching the schema includes `make schema && git status --short api/` with
  "no output" as the expected result, because CI compares the bundle byte for byte.
- Framework **versions** live in each framework schema's own `version` enum. Platform **values**
  (macOS/Windows) are duplicated across four framework schemas. Do not conflate the two: a version bump
  needs per-framework dry-runs with the new version accepted and an unlisted one rejected; a platform
  bump needs all four files checked. See `.claude/skills/saucectl-dev/SKILL.md`.
- Regression for any bundle change: `--dry-run` of the other kinds' fixtures in `.sauce/` must print no
  validation errors that `main` did not already print.

## Fixtures and examples

- Runnable configurations live in `.sauce/*.yml`; the projects they reference live in `tests/e2e/`.
- The AI authoring fixture `.sauce/authoring.yml` references a real test case in the reference
  organisation; a plan should point the tester at their own authored case instead.

## Live-service etiquette (the tester works in a shared organisation)

- Create your own assets, named `manual-<initials>-<yyyymmdd>-…` (variables `manual_<initials>_…`, since
  names allow only lowercase, digits and underscores), so leftovers are findable.
- Never delete, rename or re-suite anything you did not create in the plan. Reading a colleague's asset is
  fine; running one costs them nothing but leaves a run in their history — ask first.
- Anything that drives a browser targets the public demo site `https://www.saucedemo.com`
  (`standard_user` / `secret_sauce`).
- Shared values go at `team` scope unless a scenario is specifically about another scope; organisation
  scope is visible to every team.
- The plan ends with searches by the naming convention across every asset type, all expected to return 0.

## Verified behaviour of the AI Authoring service

The authoritative list is `specs/001-ai-test-authoring/research.md` (observations, not the OpenAPI
document, which several of them contradict). The ones a plan is most likely to need, all *observed
2026-09-05/06*:

- Authentication is HTTP Basic. The spec says bearer.
- `GET /testcases/{id}/runs` ignores its path parameter; only the `testCaseId` query parameter filters.
  The CLI always sends it; a plan checks `list-runs … | jq '[.items[].testCaseId] | unique'`.
- A run has no status field. `success` appears on the run resource ~20 s after start, a few seconds
  *before* the underlying Sauce job reads `complete`.
- Build names come back decorated: `--build X` becomes `"X - 1"` on the run and in the dashboard.
- A stored empty tunnel name (`""`) fails runs with `SC_TUNNEL_NOT_FOUND`; the CLI sends an explicit
  `null`, which also clears the stored value.
- Finished jobs expose video, logs and screenshots but no `junit.xml`; JUnit content is synthesised.
- `UTC` is not a valid schedule timezone (region/city IANA names only); cron has six fields, seconds first.
- Schedule update requires the full object; omitted optional fields are kept; explicit `null` clears.
- Variables listing needs `1 ≤ limit ≤ 200`; test cases, suites and schedules accept `limit=0` as
  count-only.
- A suite's `testCaseCount` in create/update responses lags behind the change.
- Run history outlives a deleted test case (list and detail both still resolve).
- Code export is non-deterministic: two exports of the same revision differ.
- The entitlement lives in the platform API `GET /v2/entitlements/entities/org/{orgID}?entitlements=ai_authoring.enabled`.

## Repository and CI

- `.claude/skills/`, `.specify/` and `specs/` are tracked; anything else under `.claude/` (personal
  settings) is not and must never be staged. Stage explicit paths; never `git add -A`.
- Ask before every commit and push, and stop until answered.
- Commit summaries: capitalised, imperative, ≤ 50 characters; body explains why; trailer
  `Co-Authored-By` when applicable.
- The `xctest` end-to-end CI job has been red on `main` since 2026-07-29 because the sample app's own test
  fails on the device; a red `xctest` on a PR that does not touch XCTest is not a regression.
