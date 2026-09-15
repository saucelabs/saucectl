# saucectl ai — AI Test Authoring from the CLI

`saucectl ai` brings [Sauce Labs AI Test Authoring](https://docs.saucelabs.com/) to the
command line: manage test cases that were authored in natural language (via the web UI or
the VS Code / IntelliJ plugins) and run them in the Sauce Labs cloud with CI-grade exit
codes.

> **Status: experimental.** The command group is hidden (`saucectl ai` does not appear in
> `saucectl --help`) until the feature is ready for release. The underlying backend API is
> not yet a public, versioned contract.

## Prerequisites

- **Credentials** — the usual saucectl credentials, via `saucectl configure`,
  `~/.sauce/credentials.yml`, or `SAUCE_USERNAME`/`SAUCE_ACCESS_KEY`.
- **Entitlement** — your organization needs `ai_authoring.enabled` in its plan. Every
  `saucectl ai` command checks this first and fails closed:
  - `AI Test Authoring is not enabled for your organization` → contact your Sauce Labs
    account executive.
  - `unable to verify the AI Test Authoring entitlement` → transient problem (network,
    credentials); the feature may still be in your plan.
- **A build of this branch** — the feature is not in released saucectl binaries yet:

  ```bash
  git checkout feat/ai-authoring-phase1
  go build -o bin/saucectl ./cmd/saucectl
  ./bin/saucectl ai --help
  ```

  During development, `go run ./cmd/saucectl ai …` works too.

All commands accept `--region` / `-r` (default `us-west-1`; also `us-east-4`,
`eu-central-1`).

## Finding ids

Both id types appear in app URLs and in `saucectl ai testcases list`:

| Resource | URL | id format |
|---|---|---|
| Test case | `app.saucelabs.com/test-authoring/chat/<id>` | 24 hex chars, e.g. `6a86100da038f44fd9cdaec4` |
| Test suite | `app.saucelabs.com/test-authoring/test-suites/<id>` | 32 hex chars, e.g. `bfb5e37bc38a408c89b2c4feb207944d` |

`ai run` and `ai run-suite` validate the id format and point you to the other command if
you mix them up.

## Managing test cases

```bash
saucectl ai testcases list                          # table: ID, Name, Status, Created, Creator
saucectl ai testcases list -o json                  # machine-readable
saucectl ai testcases list --search "login" --limit 10 --skip 20

saucectl ai testcases get <testCaseID>              # details incl. authored target and test URL
saucectl ai testcases get <testCaseID> -o json      # full object, incl. runSettings

saucectl ai testcases rename <testCaseID> --name "New name"
saucectl ai testcases delete <testCaseID>           # permanent, no confirmation prompt
```

`testcases` is also available as the alias `tc`.

## Running a single test case

```bash
saucectl ai run <testCaseID>
```

This starts a cloud run, waits for it to finish (polling every 15 s), prints a results
table with the Sauce Labs job link, and exits `0` if the run passed, non-zero otherwise —
so it can be a CI step as-is.

```
Started run f0620714-… of test case 6a86100da038f44fd9cdaec4 with 1 job(s)
 Name                              Status  URL
 Filter notes by keyword - …       passed  https://app.saucelabs.com/tests/4cbb0f37…
 1 of 1 job(s) passed
```

### Flags

| Flag | Meaning |
|---|---|
| `--async` | Start the run and exit immediately without waiting for the result. |
| `--build <name>` | Associate the run with a build name. The backend suffixes duplicates (`name - 2`). |
| `--tunnel-name <name>` | Run through a Sauce Connect tunnel. |
| `--browser`, `--browser-version`, `--platform` | Override the target platform (e.g. `--browser chrome --platform "Windows 11"`). |
| `--timeout <dur>` | Max time to wait for the result (default `30m`). On expiry the cloud run keeps going; the CLI prints the build URL and exits non-zero. |
| `-o text\|json` | Output format. `json` emits the final job results (or the started run(s) with `--async`). |

### Target selection

The run target is chosen in this order:

1. `--browser` / `--platform` flags, if given;
2. otherwise the test case's **authored** target (`runSettings.primaryTarget` — what you
   see in `testcases get`), which may be a desktop browser or a real mobile device;
3. otherwise the backend's default for the test case.

## Running a whole test suite

```bash
saucectl ai run-suite <testSuiteID>
```

The backend has no suite-level run endpoint, so saucectl fans the suite out client-side:
it lists the suite's test cases, starts one run per case (each with its own authored
target, unless overridden by flags), polls all runs **concurrently**, and prints one
aggregated table. The exit code is non-zero if *any* case fails — including cases that
fail to start; one broken case never prevents the others from running.

```
Running test suite "Example Test Suite" with 3 test case(s)
 Name                                    Status  URL
 New vt test case login + add backpack   passed  https://app.saucelabs.com/tests/…
 Log in test                             passed  https://app.saucelabs.com/tests/…
 Failing test                            error
 2 of 3 job(s) passed
Error: some test case jobs have failed
```

`run-suite` takes the same flags as `run`. Note that `--browser`/`--platform` overrides
apply to **every** case in the suite.

## Behavior notes & known quirks

These reflect observed backend behavior; they are handled by the CLI but useful to know:

- **Run progress lives on the run resource.** The job ids returned when a run starts are
  internal to the AI Authoring backend and cannot be looked up via the regular Sauce job
  APIs. saucectl polls `GET …/testcases/{id}/runs/{runId}`, where the real Sauce job id,
  URL, and pass/fail appear as the run progresses.
- **VDC job URLs are constructed.** The backend omits `url` for virtual-device jobs (it
  sets it for real-device jobs); saucectl builds the dashboard link from the Sauce job id.
- **Authored empty tunnel names are neutralized.** A test case saved with an *empty*
  (rather than absent) tunnel name in its run settings would fail to start with
  `SC_TUNNEL_NOT_FOUND`; saucectl detects this and clears the tunnel by sending an
  explicit `scTunnelName: null` (unless you pass `--tunnel-name`).
- **Suites can contain unrunnable drafts.** A test case with no saved revision fails to
  start with `TEST_CASE_REVISION_NOT_FOUND`; it is reported as an `error` row while the
  rest of the suite runs.
- **Device allocation takes time.** A run can sit in allocation for a few minutes before
  test steps execute; the default 30 m timeout accounts for this.

## What this feature does not do (yet)

Planned in later phases: authoring new test cases from the CLI (`ai generate`, `ai new`,
WebSocket streaming), exporting generated code (`ai targets`, `ai export`), and a
`kind: ai` config file so plain `saucectl run` can drive these runs with suites,
reporters, and artifact download.
