# Phase 0 Research: AI Test Authoring in saucectl

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Date**: 2026-09-05

The service's OpenAPI specification is served at `https://docs.saucelabs.com/oas/test-authoring-api.json`.
The documentation page renders it client-side, so fetching the HTML yields only "Loading API reference…";
the JSON must be fetched directly. It declares itself "AI Authoring API" over base path
`/ai-authoring/v1`, with 29 endpoints.

Everything below was established by **calling the live service** (Constitution VII). Four findings
contradict the published specification outright. Each is stated with what was observed, so a future
reader can re-run the observation rather than trusting this document.

---

## R-001: Authentication is HTTP Basic, not Bearer

**Decision**: Authenticate with `req.SetBasicAuth(username, accessKey)`, applied in exactly one helper.

**Observed**: The specification declares a `bearerAuth` security scheme (`type: http`, `scheme: bearer`,
`bearerFormat: JWT`) on every operation. A plain Basic-auth request to `GET /ai-authoring/v1/testcases`
returned **200** on both `us-west-1` and `eu-central-1`.

**Rationale**: Basic auth with username + access key is the platform-wide convention and is what every
other saucectl client already does. No new credential handling is needed.

**Alternatives considered**: Implementing JWT acquisition as documented — rejected because there is no
token endpoint in the specification and Basic auth demonstrably works. Confining auth to one helper means
a future tightening to JWT is a one-line change.

---

## R-002: The suite-run endpoint exists but its result cannot be followed

**Decision**: Expose `POST /testsuites/{id}/run` in the CLI as a fire-and-forget command. **Do not use it
in the runner.** The runner expands a suite into its test cases and starts each individually.

**Observed**: The route is live — a request with a non-existent suite ID returns
`404 {"error":{"code":"TEST_SUITE_NOT_FOUND"}}`, a resource-level error rather than a routing failure. Its
success response contains only `{id, orgId, teamId, userId, runCount, buildName}`: no per-case run
identifiers and no job list. Its description reads "Queues test case runs for every test case in the
suite." No `GET /testsuites/{id}/runs` endpoint exists.

**Rationale**: Without per-case run identifiers there is nothing to poll, so results could never be
collected, reported, or turned into an exit code — defeating the feature's primary purpose (User Story 1).

**Alternatives considered**:
- *Poll every test case's run history and correlate on the returned build name* — rejected as racy and
  fragile, and it requires enumerating the suite's cases anyway, so it saves nothing.
- *Expose it only under `--async`* — deferred; the plain command already covers that need.

---

## R-003: Suite membership is discoverable, which makes client-side expansion viable

**Decision**: Resolve a suite to its test cases with `GET /testcases?testSuiteId=<id>`, then start one run
per case.

**Observed**: A suite reporting `testCaseCount: 12` returned exactly `total: 12` from that query, with
matching identifiers.

**Rationale**: Gives each case its own pollable run, per-case reporting, and genuine concurrency control.

**Alternatives considered**: Requiring users to list case IDs explicitly — rejected as hostile; suites are
the natural unit of organisation. Explicit case lists remain available for precision.

---

## R-004: The two run endpoints treat their path parameter inconsistently

**Decision**: Always send `testCaseId` as a **query parameter** when listing runs. Always poll a single run
using the `testCaseId` **from the run object returned by the start request**, never the identifier
originally requested.

**Observed**:

| Request | `total` | Distinct `testCaseId` in results |
|---|---|---|
| `GET /testcases/<A>/runs` | 324 | four different test cases, none of them `<A>` |
| `GET /testcases/<A>/runs?testCaseId=<A>` | 0 | — (`<A>` genuinely has no runs) |

The path segment is decorative on the list endpoint; the query parameter is the real filter. The detail
endpoint `GET /testcases/{id}/runs/{runId}` behaves the *opposite* way — it enforces the path identifier
and returns `404 TEST_CASE_RUN_NOT_FOUND` when it is not the run's own `testCaseId`. A run identifier
taken from the unfiltered list therefore 404s on the detail endpoint, which is how this was discovered.

**Rationale**: Omitting the query parameter silently returns **every run in the organisation**. This is
invisible to manual testing because plausible rows still come back. It is encoded as SC-002 in the
specification and requires an explicit client test, since no human review would catch it.

**Alternatives considered**: Trusting the path parameter, as the specification's shape implies and as the
parameter's apparent redundancy suggests — rejected by observation. Both prior design passes over this
API described the query parameter as "redundant, not exposed"; shipping that would have been a
correctness defect.

---

## R-005: Run completion has no status field and must be inferred

**Decision**: Model per-job success as `*bool`. `nil` means "not yet reported". A run is complete when
every job has either `success` or `error` set. Poll before the first tick and short-circuit if the start
response is already terminal.

**Observed**: Neither a run nor a job carries a `status` field. In the job schema, `required` is only
`["id", "target", "name"]` — `success` is optional. Across ten completed run records, every job had
`success` populated. Descriptions say "Starts a test case run" and "Test case run created", indicating the
call returns before completion. **Confirmed by observation on 2026-09-05** — see Open-1 below for the
watched transition.

**Rationale**: The evidence says asynchronous, but the design must not bet on it. Polling first and
short-circuiting on an already-terminal response behaves correctly whether the service answers
asynchronously or synchronously, with no added latency in the synchronous case.

**Alternatives considered**: Assuming synchronous and reading `success` directly — rejected as it would
report every run as failed while jobs were still in flight. Polling the underlying Sauce job instead is
retained as the documented fallback (see **Open-1**).

---

## R-006: Job dashboard links must be derived, not read

**Decision**: Build the link as `region.AppBaseURL() + "/tests/" + sauceJobId`. Treat the response's `url`
field as an optional override only.

**Observed**: Across ten real run records, `url` was present on **7 of 13** jobs; `sauceJobId` was present
on all 13.

**Rationale**: A result without a link is a result a user cannot investigate. The chosen format is the
established one in this codebase (`internal/http/resto.go:449`, `rdcservice.go:628`,
`webdriver.go:211`).

**Alternatives considered**: Using `url` when present and omitting otherwise — rejected; a link is absent
for no user-visible reason in roughly half of cases.

---

## R-007: The error envelope carries an undocumented field holding the actual message

**Decision**: Decode `error.data[]` alongside `error.code` and `error.detail`, and include its messages in
the rendered error.

**Observed**: `GET /variables?scope=testSuite` without the matching identifier returned:

```json
{"error":{"code":"INVALID_QUERY","detail":"Invalid query string parameters.",
  "data":[{"code":"custom","path":[],
           "message":"scope-specific id is required: scope=testCase requires testCaseId; scope=testSuite requires testSuiteId."}]}}
```

The `data` array appears in no error schema in the specification.

**Rationale**: `detail` alone is "Invalid query string parameters." — useless. The one sentence that says
what to fix lives in the undocumented field. Constitution VIII requires surfacing it.

**Alternatives considered**: Decoding only the documented `{code, detail}` — rejected; it discards the
actionable text.

---

## R-008: Some stored test cases carry an empty tunnel name

**Decision**: Give the run request a hand-written `MarshalJSON` so it can emit an explicit
`"scTunnelName": null`, distinct from omitting the field.

**Observed**: Of 12 sampled test cases, 11 had `runSettings.scTunnelName` **absent** and **1 had it set to
the empty string**.

**Rationale**: `omitempty` cannot distinguish "send nothing" from "explicitly clear what is stored". An
empty stored tunnel name is not equivalent to no tunnel, so the ability to clear it must exist.

**Alternatives considered**: Relying on `omitempty` — rejected, it cannot express the distinction. Note
the *consequence* of an empty stored tunnel name was verified on 2026-09-05: the run is **rejected** with
`SC_TUNNEL_NOT_FOUND` unless `null` is sent (see **Open-3**).

---

## R-009: An organisation entitlement gates the feature, and is absent from the specification

**Decision**: Check the entitlement in the command group's pre-run hook, fail closed, and distinguish
"not included in your plan" from "could not be verified".

**Observed**: A different platform API on the same host answers this:

```
GET {APIBaseURL}/v2/entitlements/entities/org/{orgID}?entitlements=ai_authoring.enabled
200 {"level":"org","uuid":"…","entitlements":[{"name":"ai_authoring.enabled","value":true,"region":"GLOBAL"}]}
```

`orgID` comes from `iam.UserService.User(ctx).Organization.ID`, which calls
`{URL}/team-management/v1/users/me` (`internal/http/userservice.go:58`). This endpoint appears nowhere in
the authoring specification.

**Rationale**: Without it, every command fails with an opaque 401/403 for organisations lacking the
feature, and the user cannot tell an entitlement problem from a credentials problem.

**Cost, and the accepted trade-off**: This adds **two serial round-trips to every invocation**. Three
options were weighed: check eagerly (chosen — simplest, best error, ~2 extra requests); check lazily only
after a 401/403 (no happy-path cost, but the gate stops being fail-closed and error handling spreads
across every command); or cache locally (fastest, but a stale "no" is worse than two requests). The check
is skipped for `--help` and completion paths. Revisit if the latency is felt in practice.

**Alternatives considered**: No entitlement check at all — rejected; it produces an unactionable error for
exactly the users most likely to hit it. `value` was observed as a JSON boolean but is undocumented, so it
is decoded permissively (accepting `true`, `"true"`, `1`) rather than bound to `bool`.

---

## R-010: The shared HTTP client retries 404s, which this API cannot tolerate

**Decision**: Build this client's retry policy from `retryablehttp.DefaultRetryPolicy`, overriding the
shared `CheckRetry`.

**Observed**: `internal/http/client.go:21-31` deliberately retries `http.StatusNotFound` (the check is at
line 26) for APIs where a 404 signals propagation delay. This API returns 404 as an ordinary outcome —
`TEST_CASE_NOT_FOUND`, `VARIABLE_NOT_FOUND`, `FILE_NOT_FOUND`.

**Rationale**: Unpatched, every failed lookup costs four requests and roughly seven seconds. Encoded as
SC-007 and locked in by a test that counts handler invocations and asserts exactly one.

**Alternatives considered**: Accepting the shared default — rejected; it makes ordinary "not found"
answers feel like a hang. Note the retry behaviour is *desirable* in one place: the fallback in **Open-1**
polls a pre-generated Sauce job ID that may briefly 404.

---

## R-011: JUnit output would be structurally empty without synthesis

**Decision**: When a JUnit reporter is active, synthesize `Attempt.TestSuites` from the run's jobs — one
test case per job, with a failure element carrying the job's error.

**Observed**: `internal/report/junit/junit.go:30-55` builds `<testcase>` elements exclusively from
`attempt.TestSuites`. The only code that populates that field is `CloudRunner`, which downloads a JUnit
asset — gated on `report.IsArtifactRequired(..., report.JUnitArtifact)` at
`internal/saucecloud/cloud.go:643` — and parses it at line 685. An authoring runner is not a
`CloudRunner`, and AI-driven browser sessions are unlikely to emit a JUnit asset.

**Rationale**: Without synthesis, `reporters.junit.enabled: true` produces `<testsuite>` elements with
zero children — a report that appears to work and conveys nothing. Encoded as FR-006 and SC-008.

**Alternatives considered**: Documenting JUnit as unsupported — rejected, it is a primary CI integration.
Downloading and parsing a JUnit asset as `CloudRunner` does — depends on **Open-2**; synthesis works
regardless and is strictly simpler.

---

## R-012: Concurrency must be bounded, unlike the closest analogue

**Decision**: Bound concurrency with a buffered-channel semaphore sized from `sauce.concurrency`,
following the shape of `saucecloud.CloudRunner.createWorkerPool` (`internal/saucecloud/cloud.go:93`).

**Observed**: `internal/apitest/runner.go:427` — the closest structural analogue — opens an unbuffered
result channel and starts every test at once, never reading `Sauce.Concurrency`. Separately, it assigns to
a **package-level** `pollWaitTime` from inside per-suite goroutines (`runner.go:24`, `:314`, `:367`): a
data race that also lets one suite's timeout become every other suite's poll interval.

**Rationale**: An authoring run consumes real VM and device capacity, so unbounded fan-out can exhaust an
organisation's concurrency. Encoded as SC-011. `CloudRunner` itself is not reusable — it is built around
`job.StartOptions` (WebDriver job creation), whereas these runs are triggered through the authoring
service.

**Alternatives considered**: Copying apitest verbatim — rejected on both counts. This plan's poll interval
is written only by tests, and each suite's timeout is passed as an argument rather than stored globally.
`golang.org/x/sync/errgroup` was rejected because `x/sync` is only a transitive entry in `go.sum` and
unused in `internal/`; a semaphore needs no new dependency.

---

## Supporting observations

Verified, lower-consequence, but load-bearing for specific commands:

- **Pagination is well behaved.** `skip` genuinely pages (`skip=0` and `skip=3` returned disjoint
  identifiers); `limit` is uncapped to at least 1000, clamping to the result-set size. A page size of 100
  for exhaustive listing is safe.
- **`limit=0` is meaningful**, returning `{"total":187,"items":[]}` — a count-only mode. A query helper
  that skips non-positive integers makes this unreachable, so the parameter must be sent when the user
  explicitly set it rather than when its value is positive.
- **Listings are heavy.** A test case list includes every revision and every step: ~11.5 KB for one, 267 KB
  for twenty — about 13 KB per case. Exhaustive listing over the 187-case reference organisation is ~2.4 MB.
  Hence a conservative default page size and a warning at high volumes.
- **Search is a case-insensitive substring match, inside words.** `search=demo` returned "Optum Demo",
  "Demo Suite", "Sauce**demo** - Checkout flow" and "Sauce Demo Mobile". Resolving a suite by exact name
  must therefore filter client-side, and an ambiguous match must be an error rather than "first wins".
- **Labels are case-sensitive.** A real organisation returned both `Login` and `login`.
- **Scope filters require their identifier.** `GET /variables?scope=testSuite` without `testSuiteId`
  returns `400 INVALID_QUERY`; likewise `testCase` without `testCaseId`. This applies to listing, not only
  creation — worth validating client-side for a better message.
- **Confidential values are withheld as documented.** Sampled variables showed `value` populated for
  non-secret entries and absent for secret ones.
- **Code export works and reports a distinct error.** A valid request returns `{"data":{"code":"…"}}`
  (~1.3 KB for a TypeScript Playwright export). An unavailable target returns
  `404 CODE_GENERATION_TARGET_NOT_FOUND` — not the `400 CODE_GENERATION_TARGET_INVALID` the specification
  also defines. Available targets for the reference organisation: `javascript_webdriverio`,
  `typescript_webdriverio`, `python_selenium`, `java_selenium`, `csharp_selenium`, `javascript_wdi5`,
  `typescript_wdi5`, `typescript_playwright`, `javascript_playwright`.
- **Artifact identifiers must be extracted.** A step's screenshot reference is not an identifier but a
  fully pre-signed Google Cloud Storage URL (`X-Goog-Signature`, `X-Goog-Expires=86400`). The artifact
  identifier is its **last path segment before the query string**; passing that to `GET /storage/<id>`
  returned 200 with 58 KB of PNG, while a fabricated identifier returned `404 FILE_NOT_FOUND`. The
  download response carries **no `Content-Type`**, so no file extension can be inferred from it.
- **The revision-scoped run is undocumented as a path.** The run endpoint's description mentions appending
  a revision identifier, but no such path is declared. Treat it as unverified surface.
- **All three data centres serve the API**: `us-west-1`, `us-east-4`, `eu-central-1` — already present in
  `internal/region/region.go`. The shared configuration schema's region enum already lists all three, so
  no schema change is needed there. `staging` is not offered.

---

## Open questions carried into implementation

Open-1, Open-2 and Open-3 were **answered by observation on 2026-09-05** — three runs of test case
`6a88…` (a 7-step saucedemo journey, Chrome / Windows 11, `us-west-1`, ~20–25 s each).
Open-4 and Open-5 remain open, each with a contained mitigation.

- **Open-1 — RESOLVED: completion is reported on the run resource, and it is the *earliest* signal.**
  Timeline for run `db4ce5ab-7beb-4de6-8015-331c18aca742`, polled every ~4 s from t=0:

  | t (s) | run resource `jobs[0]` | underlying Sauce job |
  |---|---|---|
  | 0 | keys `id, name, sauceJobId, target` — no `success`, `error` or `isRdc` | `queued` |
  | 8 | unchanged | `in progress`, `start_time` set |
  | 15 | `isRdc` appears | `in progress` |
  | 19 | **`success: true` appears** | `in progress`, `passed: true`, `end_time` null |
  | 23 | — | `complete` |

  Consequences: (1) the start response is never terminal, so short-circuiting on it is a safety net, not
  the common path; (2) `success` and job-level `isRdc` are **absent until late** — decode `success` as
  `*bool` and read `isRdc` from `target.isRdc`, which is present from the start; (3) the Sauce job reaches
  `complete` ~4 s *after* the run reports `success`, so the R-005 fallback of polling the Sauce job would
  be slower and is not needed. A 5 s poll interval suits a ~20 s run. `url` and `error` were absent on a
  passing job.
- **Open-2 — RESOLVED: authored runs publish standard VDC job assets.** `GET /rest/v1/{user}/jobs/{id}/assets`
  on the finished job listed `video.mp4`, `selenium-server.log`, `log.json`, `automator.log` and eleven
  `NNNNscreenshot.png` files — and **no `junit.xml`**. JUnit synthesis (R-011) is therefore the only source
  of `<testcase>` elements, and artifact download via `saucecloud.JobService.DownloadArtifacts` is
  supported. Because the Sauce job completes a few seconds after the run reports success, downloads must
  tolerate a briefly-unfinished job; the shared client's 404 retry covers that window.
- **Open-3 — RESOLVED: an empty stored tunnel name breaks the run unless explicitly cleared.** Starting the
  case with `{"buildName":"…"}` and no tunnel field returned, in ~1 s,
  `400 {"error":{"code":"SC_TUNNEL_NOT_FOUND","detail":"Sauce Connect tunnel does not exist."}}`.
  Repeating with `"scTunnelName": null` returned `200` and the run completed. **Decision**: when the
  configuration carries no tunnel, the runner sends an explicit `null` (R-008's `MarshalJSON`); a configured
  tunnel is sent by name. `omitempty` alone would have made every such case unrunnable.
  **Side effect, observed afterwards**: the explicit `null` *persisted* — after the runs the test case's
  `runSettings.scTunnelName` is absent (it was `""`) and its `lastModifierUserName` is the runner. A run
  with `scTunnelName: null` therefore also clears the stored default; a run with a name presumably stores
  it. This is benign for the runner (configuration is the source of truth) but worth knowing.
- **Variables listing has no count-only mode.** `GET /variables?limit=0` returns
  `400 INVALID_QUERY` with `data[0].message = "limit: Too small: expected number to be >=1"`; the
  endpoint requires `1 ≤ limit ≤ 200`, unlike the other listings. The client rejects `--limit 0` for
  variables with that explanation.
- **Schedule cron expressions have six fields**, seconds first: live schedules carry `0 0 10 * * *` and
  `0 40 17 * * *`. The data model's "5-field" note was corrected.

**Further observations from the same runs**

- The service **decorates the build name**: `buildName: "X"` came back as `build: "X - 1"` on both the run
  and the Sauce job. Build links must be resolved through the build service *by job ID*
  (`GET /v2/builds/vdc/jobs/{jobId}/build/` returned the build with `status: success`), never by name.
- Every job the service starts carries `sauce:options.custom-data.aiAuthoring_testCaseId`, and the
  returned target capabilities gain injected `sauce:options.build` and `sauce:options.name`.
- `GET /testcases/{id}/runs?testCaseId={id}` returned exactly this case's runs (`total: 2`, one distinct
  `testCaseId`); the same path without the query parameter returned `total: 326` across three test cases.
  R-004 re-confirmed with a case that now has runs of its own.

- **Open-4 — RESOLVED 2026-09-06: the schedule update endpoint requires the required fields, merges the
  optional ones, and clears on explicit null.** Probed on a throwaway schedule:

  | Body sent to `POST /test-schedules/{id}` | Result |
  |---|---|
  | `{"settings":{"cron":"…"}}` only | `400 INVALID_BODY`, `data[]` names `name`, `settings.timezone`, `settings.runningUserId`, `testSuiteIds`, `stateName` as missing — all marked optional in the specification |
  | full required set, optional fields **omitted** | `maxRuns`, `startDate`, `endDate` **kept** their stored values |
  | full required set, `maxRuns`/`startDate`/`endDate`: **null** | all three **cleared** — although the specification marks them non-nullable |
  | `maxRuns: 0` | stored as `0`, a real value, not "unlimited" |
  | `startDate: ""` | `400` "expected date" |

  The client therefore reads the schedule, always sends name, the required settings, the full suite list
  and the state, sends an explicit `null` for every `--unset` field, and applies `--add/--remove-test-suite-id`
  client-side. A schedule observed in `RUNNING` or `ERRORED` cannot be sent back as-is (only
  `ENABLED`/`DISABLED` are accepted), so updating one requires an explicit `--state`.
- **`UTC` is not a valid schedule timezone.** `settings.timezone: "UTC"` returned `400 INVALID_BODY`
  "Value must be a valid IANA timezone." with the accepted list in `data[0].values` — region/city zones
  only; neither `UTC` nor `Etc/UTC` appears. `--timezone` is therefore required on create with no default.
- **`testCaseCount` in suite mutation responses is eventually consistent.** Creating a suite with
  `testCases: [A]` returned `testCaseCount: 0` while `GET` two seconds later returned `1` and the case's
  `testSuiteId` was set; update responses lag similarly. Commands no longer quote the count from a
  mutation response.
- **Authoring, observed end to end (2026-09-06).** `testcases generate --wait` for a five-step saucedemo
  login was accepted in ~1 s and **completed ~48 s later**; the interrupted wait printed the reattach
  hint and exited non-zero, and `generate-status --wait` reattached and reported completion. Status
  `COMPLETED` carries only `testCaseId` — no steps — so the saved test case must be fetched for them. The
  new case ran through `saucectl run` in 22 s with a JUnit entry and build link.
- **A job that cannot start is still reported through the run resource.** Running a case with
  `browserVersion: "999"` produced `success: false, error: "Unable to start your session."` within ~6 s;
  the pipeline exited 1 and the JUnit failure carried that message. The build link was `N/A`, since the
  job never existed on the Sauce side.
- **Run history outlives its test case.** After deleting a case, `GET /testcases/{id}/runs?testCaseId=`
  still returned its run and `GET /testcases/{id}/runs/{runId}` still resolved. Deletion orphans runs; it
  does not remove them. The delete confirmation says so.
- **The suite-run endpoint accepts an omitted `buildName`.** Probed 2026-09-07 on a throwaway suite with
  no members, so no jobs and no VM time: `{}`, `{"buildName": null}` and `{"buildName": "x"}` all reached
  the same business error, `400 TEST_SUITE_NO_RUN_JOBS` ("No jobs were triggered as a part of this test
  suite run"), rather than an `INVALID_BODY`. Omission is therefore valid, and the explicit null the
  client used to send was unnecessary — unlike `scTunnelName` on the test-case run endpoint, where the
  null is load-bearing (Open-3). The client now omits it, matching `RunOptions`.
- **Code export is not deterministic.** Two consecutive exports of the same revision to
  `typescript_playwright` returned 1194 and 946 bytes of different source. Reviewers should expect diffs
  between exports; the Java export named its class `SauceDemoCartTest` and the derived filename matched.
- **Open-5 — Is the generate request safe to retry?** The shared retry policy retries 5xx responses. A
  retry after a request that actually succeeded would start a second authoring task, consuming a second
  VM. *Decision needed*: a non-retrying path for that one call (recommended) versus accepting the risk.
  The same exposure applies to starting a run, and to `apitest` today, so shipping as-is is at least
  consistent.
