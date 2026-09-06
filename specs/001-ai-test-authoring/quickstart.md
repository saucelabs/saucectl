# Quickstart: Validating AI Test Authoring in saucectl

**Feature**: [spec.md](./spec.md) | **Contracts**: [cli-surface.md](./contracts/cli-surface.md), [config-kind.md](./contracts/config-kind.md)

How to prove the feature works end to end. Each section names the requirement or success criterion it
exercises, so a reviewer can trace coverage. Command shapes come from the contracts; they are not
restated here.

## Prerequisites

- Go 1.26 and a working `make build`
- `golangci-lint` **v2** — a v1 binary cannot parse `.golangci.yml`
- Node 20 plus `npm ci` in `scripts/json-schema-bundler/`, only if schema sources changed
- Sauce Labs credentials — environment variables or `~/.sauce/credentials.yml`
- An organisation **with the AI authoring entitlement**, holding at least one authored test case and one
  suite
- For the runner sections: capacity to start real jobs, which consume account minutes

```bash
make build        # ./saucectl
```

## 0. Gates before anything else

```bash
make lint && make test && make build
make schema       # only when api/ sources changed
```

For the schema, confirm the diff covers **both** the source schema and the regenerated bundle, and that
no stray comparison file was left behind. Note `git diff --exit-code` on `api/` is not a valid check
here — this change deliberately makes that diff non-empty.

## 1. Entitlement gate (FR-032)

Run any command against an entitled organisation and confirm it proceeds. Then point the check at an
organisation identifier that lacks the feature and confirm the message says the capability is not in the
plan — clearly distinct from a credentials failure. Finally, confirm `--help` works without the check
running, so help is never gated behind two network round-trips.

## 2. Read-only surface (User Story 2, FR-014–018)

Exercise listing and inspection across both `us-west-1` and `eu-central-1`: list test cases with and
without filters, inspect one with its steps shown, list labels, and list export targets.

Confirm while doing so:

- Machine-readable output is valid and mirrors what the text view shows.
- Steps render as readable one-line summaries, and an unrecognised action type degrades to its bare name
  rather than breaking the view.
- A step's screenshot column shows a short identifier, **not** a kilobyte-long signed URL.
- An empty result prints a single line and no table frame.

### 2a. The filtering trap — do not skip (SC-002)

```bash
./saucectl authoring testcases list-runs <id> -o json | jq -r '[.items[].testCaseId] | unique'
```

**Expected**: exactly one identifier, equal to `<id>`.

If other identifiers appear, the query parameter is missing and the command is returning the whole
organisation's run history. This cannot be caught by eye — the unfiltered call returns plausible rows.
Then take a run identifier from that output and fetch it individually; it must succeed, which it only
does when the request uses the run's own test-case identifier.

### 2b. Not-found latency (SC-007)

Request a well-formed but non-existent identifier and confirm the answer arrives in about the time of one
request. Four requests and several seconds means the shared 404-retry policy was not overridden.

## 3. Mutating surface (FR-019–021)

Create a suite, add and remove a member, then remove the suite. Create a schedule, suspend it, resume it,
then remove it. Confirm incremental membership changes leave unrelated members untouched, and that
combining wholesale replacement with incremental flags is rejected before any request is sent.

## 4. Confidential values (FR-022–024, SC-003)

Create a confidential variable supplying its value from a file or standard input, then read it back and
confirm the content is **never** displayed. Confirm the command that supplies a value via a flag warns
about shell-history exposure.

Then the conflict path: read a variable's token in one shell, change the variable from a second shell, and
attempt a change from the first using the now-stale token. Expect a refusal that names the command to
re-read with — not a silent overwrite.

## 5. Destructive-action confirmation (FR-037–041, SC-012)

For each of the four asset types:

| Condition | Expected |
|---|---|
| Interactive, no bypass | prompt naming what is removed and what else it affects |
| Interactive, bypass | removes without prompting |
| **Non-interactive, no bypass** | **refuses and explains** — must not hang, must not proceed |
| Non-interactive, bypass | removes |

The third row is the one worth deliberate effort: pipe from `/dev/null` or run under CI to force a
non-terminal session.

## 6. Authoring from a description (User Story 3, FR-026–029)

Author a simple journey against a public site and wait for it. Confirm actions stream as they happen with
a success or failure marker, that completion reports the new identifier and how to inspect it, and that a
failure surfaces the reason and returns a non-zero status.

Then the two resumption paths:

- **Interrupt mid-authoring.** Expect a message saying the work continues remotely, naming the command to
  reattach with. Run that command and confirm it picks the task back up.
- **Do not wait at all.** Expect a reference and instructions, with a zero status.

Confirm machine-readable output emits one final object rather than a stream of partial documents.

## 7. Export to code (User Story 4, FR-030–031)

List a test case's available targets, then export to one, to standard output and to a file. Confirm:

- The derived filename suits the language — a Java export's filename matches the public class in the
  generated source, or it will not compile.
- Exporting over an existing file is refused unless overwriting is explicitly requested.
- An unavailable target lists the valid choices rather than failing opaquely.
- An empty test case is reported as empty rather than producing an empty file.

## 8. The runner (User Story 1, FR-001–013)

Using `.sauce/authoring.yml`:

```bash
./saucectl run -c .sauce/authoring.yml --dry-run
```

Confirm it lists exactly the test cases that would run — including a suite referenced by name, resolved
to its members — and starts nothing. (Credentials are still required: the authentication check precedes
the dry-run branch.)

Then run for real, and separately with a single suite selected, a concurrency limit of one, and
asynchronously.

| Check | Expectation |
|---|---|
| Exit status | non-zero when any test fails, zero when all pass (SC-001) |
| Multi-target suite | one result row per browser or device, not one per suite (FR-003) |
| Result links | every row's link opens the corresponding job |
| Build grouping | all results share the configured build name (FR-007) |
| Concurrency | never more in flight than configured (SC-011) |
| Asynchronous | returns zero and does not wait (FR-009) |
| Unsupported settings | adding retries or tags to the config produces warnings, not silence (FR-013) |
| Interruption | reports work still running rather than reporting failure |

### 8a. JUnit must contain test cases (FR-006, SC-008)

With the JUnit reporter enabled, inspect the report:

```bash
grep -c '<testcase' <report-file>
```

**Expected**: one per executed job.

Zero means the synthesis step is missing, and the report is a set of empty containers that downstream
consumers will render as "no tests". This is the single easiest thing in the feature to get wrong,
because the file is produced and looks superficially correct.

### 8b. Virtual and real device separation

If the organisation has both, run against each and confirm results land in their respective tables with a
working build link in each. A misplaced row means the real-device flag is not being set from the job.

## 9. Flag traversal

Confirm the region flag works both before and after a subcommand — three levels of grouping make this
worth testing explicitly rather than assuming.

## Open items to settle during implementation

These are recorded in [research.md](./research.md) and must be answered by observation, not inference:

- **Open-1** — start a run and watch whether completion is ever reported, recording the latency. This
  calibrates the poll interval and confirms the completion predicate.
- **Open-2** — after a run, check whether standard job assets exist. Determines whether artifact download
  can be documented as supported.
- **Open-3** — run a test case that carries an empty stored tunnel name, with no override, and record what
  happens.

Answer Open-1 and Open-3 in the same session — both need exactly one real run.
