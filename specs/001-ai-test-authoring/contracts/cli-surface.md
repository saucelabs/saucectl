# Contract: `saucectl authoring` command surface

**Feature**: [../spec.md](../spec.md) | **Plan**: [../plan.md](../plan.md) | **Model**: [../data-model.md](../data-model.md)

The user-facing contract. 29 commands map one-to-one onto service endpoints; two (`schedules enable` /
`disable`) are conveniences over a single update endpoint.

## Root group

```
saucectl authoring [-r|--region <us-west-1|us-east-4|eu-central-1>]
```

Aliases per subgroup are given below. The root pre-run resolves region, then credentials, then verifies
the organisation entitlement — failing closed and distinguishing "not in your plan" from "could not be
verified" (FR-032). The check is skipped for help and completion paths.

**Naming rules this contract follows**, so that Constitution VI holds:

- Group names are plural, matching `builds`, `devices`, `jobs`; singular aliases are registered too.
- `create` / `update` / `delete`, not `set` — every resource is identifier-addressed with distinct create
  and update endpoints.
- Hyphenated verb-noun for secondary operations (`list-runs`, `list-code-targets`), the established
  `apit vault` idiom, keeping the tree three levels deep.
- `--skip` / `--limit` mirror the service, rather than `builds`' `--page` / `--size`.
- `-o/--out` selects output **format** (`text` | `json`) everywhere. File destinations use `-f/--filename`
  or `-d/--target-dir`, **never** `-o`.

Subgroups define **no** pre-run hook of their own — cobra runs only the closest one in the chain, so
omitting it lets the root's setup run for every descendant.

## `authoring testcases` (aliases: `testcase`, `tc`)

| Command | Args | Flags |
|---|---|---|
| `list` (`ls`) | — | `-o`, `--search`, `--start-date`, `--end-date`, `--user-id`, `--team-id`, `--test-suite-id` (repeatable), `--tag` (repeatable), `--skip`, `--limit` (default 20), `--all` |
| `get` | `<id>` | `-o`, `--revision`, `--show-steps` |
| `delete` (`rm`) | `<id>` | `--yes` |
| `rename` | `<id> <name>` | `-o` |
| `run` | `<id>` | `-o`, `--build`, `--tunnel-name`, `--target` (repeatable), `--target-json`, `--revision` |
| `list-runs` (`runs`) | `<id>` | `-o`, `--start-date`, `--end-date`, `--user-id`, `--team-id`, `--skip`, `--limit`, `--all` |
| `get-run` | `<id> <run-id>` | `-o` |
| `list-tags` (`tags`) | — | `-o` |
| `generate` | — | see below |
| `generate-status` | `<task-id>` | `-o`, `--wait`, `--poll-interval`, `--wait-timeout` |
| `code` | `<id>` | `--target`, `-f/--filename`, `-d/--target-dir`, `--force`, `-o` |
| `list-code-targets` (`code-targets`) | `<id>` | `-o` |

**`list-runs` sends `testCaseId` as a query parameter.** Not optional, not redundant: without it the
service returns every run in the organisation (research R-004, SC-002). `--all` without it would page
through the whole organisation's run history.

**`get-run` is called with the run's own `testCaseId`**, which the run object carries — not necessarily
the identifier the caller passed.

**`run --revision`** appends a revision to the request path. The service describes this but does not
declare it as a path; treat as unverified surface.

### `generate` flags

| Flag | Notes |
|---|---|
| `--name` | required, 1–255 characters |
| `--intent` / `--intent-file` | mutually exclusive; `-` reads stdin; 1–20 000 characters |
| `--max-steps` | 1–200; omitted means service default |
| `--test-suite-id` | assign the result to a suite |
| `--tag` | repeatable; ≤20 tags, each ≤60 characters |
| `--target` / `--target-json` | one required; capabilities for the authoring session |
| `--test-url` | ≤2048 characters |
| `--tunnel-name` | |
| `--generation-timeout` | service-side budget, 1m–1h |
| `--wait` | stream progress until terminal |
| `--poll-interval` | default 3s (the service recommends polling every 2–3 s) |
| `--wait-timeout` | client-side limit; `0` derives from `--generation-timeout` + 2m, else 1h2m |
| `-o` | |

**Three distinct timeouts**, deliberately separately named: the per-request HTTP timeout (internal),
`--generation-timeout` (sent to the service), and `--wait-timeout` (local, applied to the command
context). No command can wait indefinitely (FR-029).

**Progress rendering.** Poll first, then wait — an already-finished task returns immediately. Interactive
and non-interactive output differ *only* by the spinner; the step lines are identical:

```
Generation task accepted.
  Task ID:       6f2c…
  Sauce job ID:  a1b2c3d4…

  * Navigating to the login page
  ✓ go_to_url https://saucedemo.com
  ✓ input_text css=#user-name ← standard_user
  ✗ click css=#login-button  (element not interactable)
```

Under `-o json`, rendering is suppressed and one final object is emitted — partial streaming would not be
valid JSON.

**Exit behaviour.** Never `os.Exit`; return an error and let the root command exit non-zero. On
interruption or wait-timeout, print
`Generation is still running on Sauce Labs. Check progress with: saucectl authoring testcases generate-status <task-id>`
(FR-028). Completion prints the new identifier; failure surfaces the service's code and detail.

### `code` behaviour

Always resolve available targets first — one cheap request buys a real error instead of
`404 CODE_GENERATION_TARGET_NOT_FOUND`. With no `--target`: prompt interactively on a terminal, otherwise
error listing the valid choices. Register shell completion for the flag.

| Flags | Result |
|---|---|
| `-o text` (default), no file flag | raw source to stdout, so `… > test.py` works |
| `-o json`, no file flag | `{"target": "…", "code": "…"}` |
| `-f <path>` | write there; report bytes written |
| `-d <dir>` | derive the filename from the target's language prefix |
| target exists, no `--force` | refuse (FR-031) |

Filenames derive from the **language prefix**, not a fixed table, so targets added later still work:
`typescript_*` → `name.spec.ts`, `python_*` → `test_name.py`, `csharp_*` → `Name.cs`. For `java_*` the
public class name is extracted from the generated source, because `javac` requires the filename to match
it.

## `authoring testsuites` (aliases: `testsuite`, `ts`)

| Command | Args | Flags |
|---|---|---|
| `list` (`ls`) | — | `-o`, `--id` (repeatable), `--search`, `--start-date`, `--end-date`, `--user-id`, `--team-id`, `--skip`, `--limit`, `--all` |
| `get` | `<id>` | `-o` |
| `create` | — | `--name` (required), `--tag` (repeatable), `--test-case` (repeatable), `-o` |
| `update` | `<id>` | `--name`, `--tag`, `--test-case`, `--add-test-case`, `--remove-test-case`, `-o` |
| `delete` (`rm`) | `<id>` | `--yes` |
| `run` | `<id>` | `--build`, `-o` |

`update` requires at least one flag, and rejects `--test-case` combined with
`--add-test-case`/`--remove-test-case` (mutually exclusive per the model).

`run` is **fire-and-forget** by contract: the service returns only a queued count and build name, with no
per-case run identifiers, so no result polling is offered here (research R-002). Use the `kind: authoring`
runner for results.

## `authoring schedules` (aliases: `schedule`, `test-schedules`)

| Command | Args | Flags |
|---|---|---|
| `list` (`ls`) | — | `-o`, `--id`, `--search`, `--start-date`, `--end-date`, `--user-id`, `--team-id`, `--test-suite-id`, `--skip`, `--limit`, `--all` |
| `get` | `<id>` | `-o` |
| `create` | — | `--name`, `--cron`, `--timezone` (required), `--running-user-id`, `--test-suite-id` (≥1), `--state`, `--start-date`, `--end-date`, `--max-runs`, `--tunnel-name`, `--build`, `-o` |
| `update` | `<id>` | as `create`, all optional, plus `--add-test-suite-id`, `--remove-test-suite-id`, `--unset` |
| `enable` / `disable` | `<id>` | `-o` |
| `delete` (`rm`) | `<id>` | `--yes` |

`--running-user-id` defaults to the caller's own identifier. `--timezone` is **required** on create — the
service rejects `UTC` and accepts region/city IANA zones only. `--state` is accepted case-insensitively.
`update` (and `enable`/`disable`) performs read-modify-write and sends the complete schedule — name,
settings, suites, state — because the service rejects a body missing any required field; omitted optional
fields are kept and `--unset` sends an explicit `null`, which is the only way to clear one (research
Open-4, resolved).

## `authoring variables` (aliases: `variable`, `var`)

| Command | Args | Flags |
|---|---|---|
| `list` (`ls`) | — | `-o`, `--scope`, `--test-suite-id`, `--test-case-id`, `--search`, `--skip`, `--limit`, `--all` |
| `get` | `<id>` | `-o` |
| `create` | — | `--scope`, `--name`, `--description`, `--secret`, `--value`, `--value-from-env`, `--value-from-file`, `--test-suite-id`, `--test-case-id`, `-o` |
| `update` | `<id>` | `--name`, `--description`, `--secret`, the three value flags, `--expected-last-update`, `-o` |
| `delete` (`rm`) | `<id>` | `--expected-last-update`, `--yes` |

**Scope pairing is validated client-side for both `list` and `create`**: `testSuite` requires
`--test-suite-id`, `testCase` requires `--test-case-id`, and neither is accepted for `org`/`team`.

**Value sourcing** (FR-023) — exactly one of:

| Source | Behaviour |
|---|---|
| `--value-from-env NAME` | read the environment variable; error if unset or empty |
| `--value-from-file <path>` | read the file; `-` reads stdin, refused when stdin is a terminal |
| `--value` | works, but its help text warns it is visible in shell history and the process list; combining with `--secret` also logs a warning |
| none, interactive | masked prompt when `--secret`, plain prompt otherwise |
| none, non-interactive | error naming all the ways to supply a value |

One trailing newline is stripped from file and stdin input, so `echo secret \| …` behaves as expected.
A confidential value is blanked before rendering and displayed as `<secret>` regardless of what the
service returns.

**Concurrency**: `update` and `delete` default to read-then-write; `--expected-last-update` pins a token
for strict callers. There is deliberately **no** `--force`: it could only be implemented as "re-read and
re-send", which is already the default, so it would imply a safety property it does not have. A `412`
surfaces as a conflict message naming the command to re-read with.

## `authoring download-artifact` (alias: `artifact`)

```
saucectl authoring download-artifact <artifact-id> -f <path> [--force]
```

`<artifact-id>` is the **last path segment before the query string** of a step's screenshot URL, as shown
by `testcases get --show-steps`. `-f` is required because the response carries no content type from which
an extension could be inferred.

## Output contract

| Aspect | Rule |
|---|---|
| Format | `-o text` (default) or `-o json`; validated before any request |
| Tables | shared style from `internal/tables`; header, per-row, footer `showing N of M <resource>` |
| Empty listing | `No <resource> found.` then return — no empty table |
| Detail views | two-column property/value, following `devices get` |
| Dates | best-effort humanised, falling back to the raw value — **except** a variable's `lastUpdate`, shown raw because users paste it back |
| Errors | the service's machine-readable code, its detail, and any per-field messages from the undocumented `error.data[]` |
| Not found | answered in one request; the shared 404-retry policy is overridden (SC-007) |

## Destructive-action contract (FR-037–041)

| Situation | Behaviour |
|---|---|
| Interactive, no `--yes` | prompt, stating what is removed and what else it affects |
| Interactive, `--yes` | remove without prompting |
| Non-interactive, no `--yes` | **refuse** and explain — never hang, never proceed unconfirmed |
| Non-interactive, `--yes` | remove |

Identical across all four asset types, so the behaviour is learned once. This is a deliberate departure
from `storage delete`, justified in the plan's Complexity Tracking.
