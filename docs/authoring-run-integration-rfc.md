# RFC: `kind: authoring` support in `saucectl run`

Status: **scoping / not started** (revision 2 — supersedes the
suite-level-run approach in revision 1)
Related: `internal/authoring`, `internal/http/authoring.go`, `internal/cmd/author`

## Goal

Let a customer point `saucectl run -c config.yml` at a config file that
references an AI-authored test suite (created via `saucectl author` /
`saucectl author sync`) and have it behave like every other framework: trigger
the run, wait for results, report pass/fail through the same reporters
(junit, json, spotlight, buildtable), and exit non-zero on failure — with
client-side concurrency control matching what every other framework already
does (e.g. 20 test cases, concurrency 10 → first 10 run immediately, each
additional one starts as soon as a slot frees up).

## Revision note

Revision 1 of this doc proposed calling `POST /testsuites/{id}/run` (one
opaque call per suite, returning only `{id, runCount, buildName}`) and
bridging to individual job results through the Builds API. That bridge is
real and documented, but it added a timing-dependent polling layer and a
`build_source` (vdc/rdc) guess that a suite could not resolve for on its own.

Better approach, per discussion: **run test cases individually**, not the
suite as a whole. This removes the Builds-API dependency and the timing risk
entirely, and it's also the only way to get real client-side concurrency
control, since `RunTestSuite` doesn't expose a concurrency parameter and we
have no way to throttle "everything in the suite, however many that is."

## Design

### Step 1: enumerate the suite's test cases (new API method needed)

Neither `internal/authoring.Service` nor the HTTP client currently has a way
to list the individual test cases in a suite — `author`/`author sync` never
needed this, since they track membership themselves via the lockfile. The
run path needs to ask Sauce directly (not trust a local lockfile that may be
stale or absent on a CI box that never ran `author`), using the already
-documented list filter:

```
GET /ai-authoring/v1/testcases?suite={testSuiteId}&skip=&limit=
```

New interface method:

```go
ListTestCases(ctx context.Context, opts ListTestCaseOptions) ([]TestCase, int, error)
// ListTestCaseOptions{ TestSuiteID string; Skip, Limit int }
// returns (page of test cases, total count, error) -- paginate until exhausted
```

### Step 2: run each test case individually (new API method needed)

`internal/http/authoring.go` currently only wraps `RunTestSuite`. The
per-test-case endpoint,`POST /testcases/{id}/run`, is not wrapped yet, and
it's the one that actually solves the reporting problem: per the spec we
pulled earlier, its response includes a `jobs[]` array with `id`,
**`sauceJobId`** ("pre-generated Sauce Labs job ID... matches the actual
Sauce Labs job ID"), `target`, `name`, `url`, **`isRdc`**, `success`, `error`
— i.e. real, immediately-usable job identifiers, synchronously, no polling
required just to discover what jobs exist. This is exactly what `apitest`
gets from `RunAllAsync`/`RunTagAsync` (`AsyncResponse{EventIDs}`), and it's
what makes the whole reporter pipeline work.

New interface method:

```go
RunTestCase(ctx context.Context, testCaseID string, opts RunTestCaseOptions) (TestCaseRun, error)
// RunTestCaseOptions{ BuildName string; Targets []map[string]interface{}; SCTunnelName string }
// TestCaseRun{ ID, TestCaseID, Jobs []TestCaseJob }
// TestCaseJob{ ID, SauceJobID, Target, Name, URL string; IsRDC, Success bool; Error string }
```

### Step 3: bounded concurrency over test cases, mirroring `CloudRunner`

This codebase already has the exact worker-pool pattern this needs —
`internal/saucecloud/cloud.go`'s `createWorkerPool`/`runJobs`/`collectResults`
(used by cypress, playwright, etc.): N goroutines consume from a buffered
channel of pending work, one item at a time, and results stream back on a
second channel as each completes, so a fast worker immediately picks up the
next item rather than waiting for the whole batch. That's precisely "20 test
cases, concurrency 10 → first 10 run, more start as slots free up" — no new
concurrency primitive needs inventing, just the same shape applied to test
cases instead of upload-and-run job options:

```
testCaseIDs := ListTestCases(suiteId)          // Step 1
workCh := bounded channel, size = testCaseIDs
resultsCh := channel

spin up `concurrency` goroutines, each:
  for id := range workCh:
    run := RunTestCase(id, ...)               // Step 2 -- returns jobs[] immediately
    for each job in run.Jobs:
        finalJob := JobService.PollJob(job.SauceJobID, realDevice: job.IsRDC)
    resultsCh <- result{testCaseID, run.Jobs, polled statuses}

main goroutine feeds all testCaseIDs into workCh, closes it,
drains len(testCaseIDs) results from resultsCh, feeds report.Reporter.Add()
```

One worker "slot" is occupied by a test case from the moment it's submitted
until every job that test case produced has finished polling — matching your
description exactly, and reusing `job.Service.Job`/`PollJob` untouched.

### The `--platform` question

The original reason I raised `vdc`/`rdc` as a concern was resolving
`build_source` for the Builds-API bridge — that requirement disappears
entirely with this design, since each job in `RunTestCase`'s response already
carries its own `isRdc` flag. We can read that per job and pass the right
`realDevice` value into `job.Service.Job`/`PollJob` directly, with no global
flag needed for that purpose.

That said, `RunTestCase`'s request takes a `targets[]` array (plural), which
implies a single test case can potentially run against more than one target
in one call — possibly a mix of vdc and rdc jobs from a single test case. If
so, a `--platform` flag could still be useful, just for a different reason
than originally proposed: as a run-time filter/override on which of the test
case's configured targets to actually execute (e.g. "only run the RDC
targets from this suite right now"), rather than something needed to resolve
job lookups. Worth confirming with a live test case that has multiple targets
configured before deciding whether `--platform` is a filter, an override, or
turns out to be unnecessary.

### Config shape

Unchanged from revision 1 — still just a suite reference:

```yaml
apiVersion: v1
kind: authoring
sauce:
  region: us-west-1
  concurrency: 10
  metadata:
    tags: [regression]
    build: "$BUILD_ID"
suites:
  - name: regression
    testSuiteId: 11111111-1111-1111-1111-111111111111
```

`sauce::concurrency` now maps cleanly (revision 1 flagged this as a
non-starter under the suite-run design — it's no longer a problem, since
concurrency is enforced client-side over individual test case runs).
`sauce::retries` can also map cleanly now: a failed test case can simply be
resubmitted via `RunTestCase` again, the same way other runners retry a
failed job.

### New/changed pieces

- **`api/v1alpha/framework/authoring.schema.json`** (+ add `"authoring"` to
  the `kind` enum in `api/saucectl.schema.json` via
  `scripts/json-schema-bundler`).
- **`internal/config`**: `Project`/`Suite` structs (name + testSuiteId) and
  an `authoring.Kind` constant, matching every other framework package.
- **`internal/authoring`**: add `ListTestCases` and `RunTestCase` to the
  `Service` interface (Steps 1–2 above).
- **`internal/http/authoring.go`**: implement both against the live API.
- **A new package**, e.g. `internal/authoringrun`, with a `Runner` built
  around the worker-pool pattern in Step 3, reusing `job.Service` for polling
  and feeding `report.Reporter.Add()`/`Render()` exactly like every other
  runner.
- **`internal/cmd/run/authoring.go`** + a new dispatch case in `run.go`'s
  `Run()`, following `apitest.go` as a template.

### What maps cleanly vs. what doesn't

Maps cleanly now: `sauce::concurrency`, `sauce::retries`,`--select-suite`,
`--dry-run`, `--async`, region/credentials plumbing, reporters.

Still an open question: whether `RunTestCase` lets you override
`runSettings.target` per invocation or only uses whatever was baked in at
authoring time. If it's authoring-time-only, then `--env`-style per-run
overrides (which other frameworks support) won't apply here — worth
confirming directly rather than assuming.

## Suggested phasing

1. **Spike** (half day): call `POST /testcases/{id}/run` against a couple of
   real test cases in a suite with your live credentials. Confirm the
   `jobs[]` shape (field names, whether `sauceJobId` really is pollable
   immediately via the existing Jobs API), and whether a single test case can
   produce multiple jobs/targets in one call. This is cheaper and lower-risk
   than the spike revision 1 proposed, and directly de-risks Steps 1–2 below.
2. **`internal/authoring` + `internal/http/authoring.go`**: add
   `ListTestCases` and `RunTestCase`, with unit tests mirroring
   `internal/http/authoring_test.go`'s existing style.
3. **Config + schema plumbing**: new kind, schema file, `config.Describe`
   recognizes it.
4. **`internal/authoringrun` runner**: worker pool per Step 3, modeled on
   `internal/saucecloud/cloud.go`'s `createWorkerPool`/`runJobs` shape rather
   than `apitest`'s (apitest fires everything at once with no client-side
   cap; we need the cap). Unit tests with a mocked `authoring.Service` and
   `job.Service`.
5. **`internal/cmd/run/authoring.go`** + dispatch wiring.
6. **Tie-in with `author sync`**: emit the `config.yml` above as part of
   sync, so the full loop (author → suite → config.yml → `saucectl run`)
   works without hand-writing config.

This removes the two open risks that made revision 1's estimate uncertain
(build-name timing, `build_source` guessing) at the cost of two new API
client methods instead of one — a better trade, and the spike in step 1 is
simpler to run than the one revision 1 called for.
