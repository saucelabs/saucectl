# Contract: `kind: authoring` configuration

**Feature**: [../spec.md](../spec.md) | **Plan**: [../plan.md](../plan.md) | **Model**: [../data-model.md](../data-model.md)

The second user-facing contract: a new configuration kind so authored suites run through `saucectl run`
alongside the existing frameworks.

## Configuration shape

```yaml
apiVersion: v1alpha
kind: authoring
sauce:
  region: us-west-1
  concurrency: 5
  metadata:
    build: nightly-$GITHUB_RUN_ID
  tunnel:
    name: my-tunnel
defaults:
  timeout: 30m
artifacts:
  download:
    when: fail
    match: ["*.mp4", "log.json"]
    directory: ./artifacts
reporters:
  junit:
    enabled: true
suites:
  - name: Regression on Chrome
    testSuiteName: Checkout Regression
    targets:
      - capabilities:
          browserName: chrome
          platformName: "Windows 11"
  - name: Two specific cases
    testCases: [<test-case-id>, <another-test-case-id>]
    # no targets -> each test case's own stored run targets apply
```

A committed fixture lives at `.sauce/authoring.yml`, matching the per-kind convention already followed by
`cypress-10.yml`, `playwright.yml`, `espresso.yml` and the rest.

## Suite reference — exactly one of three

| Field | Meaning |
|---|---|
| `testSuiteId` | a suite's identifier (dashless 32-hex) |
| `testSuiteName` | a suite's exact name, resolved at run time |
| `testCases` | an explicit list of test case identifiers (24-hex) |

Enforced as a `oneOf` in the schema **and** in `Validate`, because schema validation is advisory (below).

`testSuiteName` resolution: the service's `search` parameter is a case-insensitive substring match that
matches inside words — `demo` returns "Optum Demo", "Demo Suite", "Sauce**demo** - Checkout flow" and
"Sauce Demo Mobile". The exact match is therefore made client-side, and an ambiguous or absent match is a
hard error rather than "first result wins".

`tags` narrows a referenced suite's cases; it is ignored when `testCases` is given.

`targets` is optional — when omitted, each test case's stored run targets apply. A run fails with
`NO_RUN_TARGETS` only when neither exists.

## Field mapping

| Configuration | Sent as | Notes |
|---|---|---|
| `sauce.metadata.build` | `buildName` | defaults to `build-<timestamp>`; **truncated to 100 characters** — the per-case run endpoint caps at 100, the suite endpoint at 255 |
| `sauce.tunnel.name` | `scTunnelName` | validated for readiness with the no-op tunnel filter, **not** the v2alpha filter, which exists only for API testing |
| `sauce.concurrency` | worker pool size | |
| `suites[].targets[].capabilities` | `targets[].capabilities` | passed through untouched |
| `suites[].timeout`, `defaults.timeout` | poll deadline | per-suite, passed as an argument — never stored globally |

**Not expressible, and therefore warned about rather than silently dropped** (FR-013): `sauce.tunnel.owner`,
`sauce.retries`, `sauce.metadata.tags`, `--fail-fast`. Warnings are emitted from `Validate`, so they fire
whether the value came from YAML or from a flag.

## Inherited command-line flags

Because configuration is unmarshalled through viper and `run`'s flags bind to viper paths, these work on
this kind with no additional code: `--build`, `--tunnel-name`, `--tunnel-timeout`, `--ccy`, `--dry-run`,
`--select-suite`, `--async`, `--artifacts.download.*`, `--artifacts.cleanup`, `--reporters.junit.*`,
`--reporters.json.*`.

The same mechanism means `--tags`, `--retries`, `--root-dir` and `--sauceignore` will also populate fields
this kind cannot honour — which is why the warnings above must live in `Validate`.

`--dry-run` still requires credentials: the authentication check runs before the dry-run branch.

## Execution contract

1. Validate tunnel readiness.
2. Resolve every suite to a concrete list of test cases.
3. Under `--dry-run`, print the resolved cases and stop — resolution is read-only, so a dry run can show
   exactly what would execute.
4. Start one run per test case, bounded by `sauce.concurrency`.
5. Poll each run to completion, bounded by the suite timeout.
6. Emit **one result per job** — one per browser or device, not one per suite.
7. Render through the shared reporters; return `0` only when every result passed.

**Result fields that carry weight**:

| Field | Why |
|---|---|
| `RDC` | routes a result into the virtual-device or real-device table, each with its own build link |
| `TimedOut` | a timed-out run is counted as an *error* only if this is set; state alone reads as merely unfinished |
| `BuildURL` | obtained via the build service by job identifier, so the reporter's build link resolves |
| `URL` | derived from the region's app URL and the job identifier — the service's own field is unreliable |

**JUnit content is synthesized** when a JUnit reporter is active: one test case per job, with a failure
element carrying the job's error. Without this the report contains empty containers (research R-011,
FR-006, SC-008).

`--async` returns `0` — results are unknown by definition.

## Schema contract

New file `api/v1alpha/framework/authoring.schema.json`, referencing the shared `artifacts`, `reporters`
and `sauce` subschemas, declaring the `apiVersion` and `kind` constants and the `suites` array with its
`oneOf` reference rule. Two edits to `api/global.schema.json`: add `authoring` to the `kind` enum, and
append one `if kind == authoring / then $ref` branch.

Because targets are free-form capabilities rather than a constrained platform enum, this schema **avoids
the four-file platform-enum duplication** that the other frameworks carry — where one of the four is
silently optional and a fifth enum lurks further down the same file. There is nothing here to keep in
sync.

No change is needed to the shared region enum: it already lists all three data centres the service
offers.

**Regeneration**: `make schema` rewrites the bundle; commit the source change and the regenerated bundle
together. Continuous integration compares byte-for-byte. To check without overwriting, write the fresh
copy **outside** the repository — the path CI uses is not gitignored and would otherwise linger as an
untracked duplicate.

**Validation is advisory.** Configuration is validated against the schema published on the default branch,
not the local file, and it fails soft. So `kind: authoring` will report validation errors locally until
this merges, and — more importantly — the Go `Validate` function is the real contract. Every rule above
is enforced there, not only in the schema.
