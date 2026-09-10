# Phase 1 Data Model: AI Test Authoring in saucectl

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Research**: [research.md](./research.md)

All entities are owned by the remote service; saucectl holds none of this state. The model below is the
local representation, in `internal/authoring`. Field names are the wire names.

## Entity relationships

```text
Organisation
 ├── Variable            (scope: org | team | testSuite | testCase)
 ├── TestSuite  1 ──── * TestCase          (a case belongs to at most one suite)
 │                        ├── 1 ──── * Revision
 │                        │              └── 1 ──── * Step ──── 0..1 Artifact (screenshot)
 │                        └── 1 ──── * Run
 │                                       └── 1 ──── * RunJob   (one per target)
 └── TestSchedule * ──── * TestSuite       (a schedule triggers one or more suites)
```

## TestCase

A saved, reusable test produced from a plain-language description.

| Field | Type | Notes |
|---|---|---|
| `id` | string | 24-character hex object identifier |
| `orgId` | string | owning organisation |
| `name` | string | 1–255 characters |
| `tags` | []string | **case-sensitive** — an organisation may hold both `Login` and `login` (research: Supporting observations) |
| `testSuiteId` | string | optional; at most one suite |
| `revisions` | []Revision | full history, **returned even by list endpoints** — the reason a listing is ~13 KB per case |
| `runSettings` | RunSettings | defaults for where and how it runs |
| `creationDate`, `lastUpdateDate` | string | see *Timestamps* below |
| `creatorUserId`, `creatorUserName` | string | name may be JSON `null` |
| `lastModifierUserId`, `lastModifierUserName` | string | name may be JSON `null` |

**Derived accessor**: `LatestRevision() (Revision, bool)` — the last element, or `false` when empty. A
test case with no revisions is legitimate and must not be exported (spec: Edge Cases).

## Revision

A point-in-time version of a test case.

| Field | Type | Notes |
|---|---|---|
| `id` | string | |
| `intent` | string | the user's original description |
| `steps` | []Step | ordered |
| `discoveredIntent` | string | what the agent concluded the intent was |
| `description` | string | |
| `reasoning` | []Reasoning | `{title, description}` entries |

## Step and Tool

| Field | Type | Notes |
|---|---|---|
| `id` | string | unique within the revision |
| `tool` | Tool | the action performed |
| `result` | *StepResult | `{success, message}`; absent while unknown |
| `screenshotUrl` | string | a **pre-signed URL**, not an identifier — see *Artifact* |

`Tool` is `{type, args}` where `args` is held as **raw JSON**:

```go
type Tool struct {
	Type ToolType        `json:"type"`
	Args json.RawMessage `json:"args"`
}
```

**Validation / invariants**: twelve tool types are documented today — `go_to_url`, `switch_window`,
`enter_key_submit`, `pause`, `click`, `input_text`, `scroll_document`, `scroll_element`, `assert`,
`select`, `in_shadow_root`, `finish`. The set is expected to grow.

**Why raw JSON, not twelve typed variants** — three independent reasons:

1. `args.matcher` is a **string** for `switch_window` but an **object** for `assert`. One flattened struct
   is therefore *incorrect*, not merely lossy.
2. `in_shadow_root.args.tool` is **recursive** — a tool nesting another tool.
3. saucectl only ever *reads* steps. The operations are "render one line" and "pass through unchanged".

Typed variants would cost roughly 250 lines and would silently discard data the day a thirteenth type
ships. Presentation is handled by two accessors — `Summary()` returning e.g. `click css=#submit` and
degrading to the bare type name for unknown types, and `Reasoning()` — which cover the display need and
fail soft. Typed variants can be added additively later without changing the wire type.

## RunSettings

Defaults stored on a test case.

| Field | Type | Notes |
|---|---|---|
| `testUrl` | string | max 2048 characters |
| `scTunnelName` | string | **may be the empty string**, which is not the same as absent (research R-008) |
| `primaryTarget` | Target | |
| `runTargets` | []Target | used when a run supplies no targets of its own |
| `lastBuildName` | string | |

> **Asymmetry worth guarding against**: a *stored* test case carries `primaryTarget` + `runTargets`
> (plural), whereas an authoring *request* carries `target` (singular). These are two distinct types and
> must not be conflated.

`Target` is `{capabilities: map[string]any, isRdc: bool}`. Capabilities are free-form W3C WebDriver
capabilities, passed through untouched — which is also why the configuration schema needs no
`platformName` enum and so avoids the four-file duplication that affects the other frameworks.

## TestSuite

| Field | Type | Notes |
|---|---|---|
| `id` | string | **dashless 32-character hex UUID** — a different shape from a test case's 24-hex identifier |
| `name` | string | 1–255 characters |
| `tags` | []string | each 1–100 characters |
| `testCaseCount` | int | matches `GET /testcases?testSuiteId=` exactly (research R-003) |
| `orgId`, `teamId` | string | |
| `creationDate`, `lastUpdate` | string | |

**Update semantics**: `testCases` (replace wholesale) is **mutually exclusive** with `addTestCases` /
`removeTestCases` (incremental). The command layer must reject a combination rather than let the service
decide.

## TestSchedule

| Field | Type | Notes |
|---|---|---|
| `id` | string | |
| `name` | string | 1–255 characters |
| `settings` | ScheduleSettings | |
| `state` | ScheduleState | observed runtime state |
| `testSuiteIds` | []string | at least one required |

`ScheduleSettings`: `cron` (6-field, seconds first — observed `0 0 10 * * *` on live schedules), `timezone` (IANA), `runningUserId` (defaults to the caller's own
identifier) are **required**; `startDate`, `endDate`, `maxRuns`, `scTunnelName`, `buildName` optional.

**State transitions**:

```text
            ┌──────────── enable ────────────┐
            │                                │
        DISABLED ◄──── disable ──────── ENABLED ──── triggers ───► RUNNING
            ▲                                │                        │
            └──── maxRuns reached / ─────────┴────── failure ────► ERRORED
                  endDate passed
```

`ENABLED` and `DISABLED` are the only states a user sets. `RUNNING` and `ERRORED` are observed. Reaching
`maxRuns` or passing `endDate` disables the schedule.

**Update semantics** (research Open-4, resolved 2026-09-06): the service rejects a body missing any required
field with `INVALID_BODY`, keeps omitted optional fields, and clears an optional field on explicit `null` —
including `maxRuns`, `startDate` and `endDate`. The command layer performs read-modify-write, always sends
name, required settings, suite list and state, and sends `null` for anything the user unsets.

## Variable

| Field | Type | Notes |
|---|---|---|
| `id` | string | |
| `scope` | enum | `org` \| `team` \| `testSuite` \| `testCase` |
| `testSuiteId` | string | **required** when scope is `testSuite`, forbidden otherwise |
| `testCaseId` | string | **required** when scope is `testCase`, forbidden otherwise |
| `name` | string | 1–255 characters, pattern `^[a-z0-9_]+$` |
| `description` | string | max 1000 characters |
| `isSecret` | bool | |
| `value` | string | **never returned when `isSecret`** — verified |
| `lastUpdate` | string | the concurrency token; see below |

**Validation**: the scope/identifier pairing is enforced by the service on **listing as well as creation**
— `?scope=testSuite` without `testSuiteId` returns `400 INVALID_QUERY`. Validate client-side for a better
message.

**Concurrency control**: changing or removing a variable requires `expectedLastUpdate`, echoed from a
prior read — in the **body** for a change, in the **query string** for a removal. A mismatch returns
`412`. The default behaviour is read-then-write; a flag pins a specific token for strict callers.

## Run and RunJob

| Run field | Type | Notes |
|---|---|---|
| `id` | string | |
| `testCaseId` | string | **the value to poll with** — not the identifier originally requested (research R-004) |
| `build` | string | groups results |
| `jobs` | []RunJob | one per target |
| `creationDate`, `testUrl` | string | |

| RunJob field | Type | Notes |
|---|---|---|
| `id` | string | internal to the authoring service; **not** resolvable via the job APIs |
| `sauceJobId` | string | the real Sauce job identifier — always present |
| `target` | Target | |
| `name` | string | |
| `url` | string | **unreliable** — present on 7 of 13 sampled jobs; derive instead |
| `isRdc` | bool | determines which results table a row lands in |
| `success` | ***bool** | `nil` = not yet reported; the crux of the design |
| `error` | string | infrastructure or assertion failure text |

**Derived state** — there is no `status` field anywhere:

| Predicate | Definition |
|---|---|
| `RunJob.Done()` | `success != nil \|\| error != ""` |
| `RunJob.Passed()` | `success != nil && *success` |
| `Run.Done()` | non-empty `jobs` **and** every job `Done()` |

Mapping to saucectl's shared result states (`internal/job/job.go:24-33`): passed → `StatePassed`;
terminal but not passed → `StateFailed`; not terminal, or asynchronous → `StateInProgress`; timed out →
`StateInProgress` **plus** `TimedOut: true`.

> That last pairing is not cosmetic. `internal/report/table/table.go:97-120` counts
> `!job.Done(status) && !TimedOut` as *in progress* but `TimedOut` as an **error**. A timed-out run is
> only reported as a failure if `TimedOut` is set.

`RDC` must also be set correctly on each result: `buildtable`'s `Add` routes on it into separate virtual
and real-device tables, each with its own build link
(`internal/report/buildtable/buildtable.go:32-83`).

## Artifact

A file captured during authoring — in practice, step screenshots.

**The identifier is not a field.** A step's `screenshotUrl` is a fully pre-signed Google Cloud Storage URL
carrying `X-Goog-Signature` and `X-Goog-Expires=86400`. The artifact identifier is its **last path
segment before the query string**; that value passed to `GET /storage/<id>` returns the file, while a
fabricated one returns `404 FILE_NOT_FOUND`.

Consequences: renderings show the extracted identifier, never the ~1 KB signed URL; the download response
carries **no `Content-Type`**, so a destination filename must be supplied rather than inferred; and the
service endpoint is preferred over the signed URL because it uses Sauce credentials and does not expire.

## Cross-cutting representation decisions

**Timestamps are `string`, not `time.Time`.** Load-bearing: a variable's `lastUpdate` must be echoed back
byte-for-byte as a concurrency token, and normalising through `time.Time` re-formats it (fractional-second
precision, offset spelling), which can silently fail the comparison. Strings also make machine-readable
output an exact mirror of the service, and reduce an unexpected format to a display concern rather than a
command failure. Presentation uses a best-effort parse that falls back to the raw value — except a
variable's own `lastUpdate`, which is shown raw because users paste it back.

**Envelope and pagination.** Every success response is wrapped `{"data": …}`; listings are
`{"data":{"items":[…],"total":N}}`. One generic envelope type and one `List[T]` cover all of it, with a
page-walking helper bounded to a maximum page count so a service that ignored `skip` could not loop
forever. `limit=0` is a meaningful count-only mode, so the parameter is sent when the user set it
explicitly rather than when its value is positive.

**Errors.** A single `APIError{HTTPStatus, Code, Detail, Items}` keyed on the service's machine-readable
`Code`, with code-valued sentinels comparable through `errors.Is`. `Items` captures the undocumented
`error.data[]` array that holds the actionable message (research R-007). One type plus sentinels avoids a
per-endpoint status-to-error mapping and still surfaces unknown future codes in full.
