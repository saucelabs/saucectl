# Manual Test Plan: AI Test Authoring (PR #1100)

**Feature**: [spec.md](./spec.md) | **Quickstart**: [quickstart.md](./quickstart.md) | **Research**: [research.md](./research.md) | **Date**: 2026-09-06

## How to use this plan

- Run **every** scenario, **in order**. Later parts use assets created in earlier ones.
- Each scenario has numbered **Steps** (exact commands) and an **Expected** list. A scenario passes only
  when every Expected item holds. Record pass / fail / not run per scenario ID.
- Commands use shell variables for things you create along the way (`$TC`, `$SUITE`, …). Part 0 sets them
  up; whenever a step says **Record**, export the value before moving on.
- `a` is a shell function defined in Part 0 that stands for `./saucectl authoring --disable-usage-metrics`.
  Pipeline scenarios spell out `./saucectl run` in full.
- Run everything from a real terminal (not under `script`, CI or a pipe). Some scenarios deliberately pipe
  stdin from `/dev/null` to simulate a pipeline; they say so.
- Until the PR merges, every `./saucectl run` with this kind prints one red advisory block
  ("value must be one of … in /kind"). That is expected and is checked explicitly in scenario 10.16.
- Two behaviours are **known gaps** that will be fixed on the branch; the scenarios that hit them
  (1.4 and 8.9) describe the current behaviour as the expected result. Do not file them as new defects.

Time: about a working day for everything, including roughly ten authoring sessions and ten pipeline runs
of ~30 s of VM time each.

---

## Part 0 — Setup

### 0.1 Build the binary from the branch

```bash
git fetch origin 001-ai-test-authoring
git checkout 001-ai-test-authoring
make build
./saucectl --version
```

**Expected**
- `./saucectl` exists. The version prints `saucectl version 0.0.0+unknown` (local builds are never stamped;
  this is normal).

### 0.2 Credentials and entitlement

You need an account in an organisation that has the AI authoring entitlement. Either run `./saucectl configure`
or export `SAUCE_USERNAME` and `SAUCE_ACCESS_KEY`.

```bash
./saucectl authoring --disable-usage-metrics testcases list --limit 1
```

**Expected**
- A one-row table and the footer `showing 1 of N test cases`. If you see "not included in your Sauce Labs
  plan" you are in the wrong organisation.

### 0.3 Shell setup for the rest of the plan

```bash
a() { ./saucectl authoring --disable-usage-metrics "$@"; }
export I=<your initials, lowercase, e.g. vt>
export W=/tmp/authoring-manual && mkdir -p "$W"
export TODAY=$(date -u +%Y-%m-%d)
```

Every asset you create is named `manual-$I-…` (variables: `manual_$I_…`) so it can be found and deleted at
the end. Never delete, rename or move a test case, suite, schedule or variable you did not create in this plan.

---

## Part 1 — Access and region

### 1.1 Help works without any network access

**Steps**
1. Disconnect from the network (or `export SAUCE_ACCESS_KEY=bogus` temporarily).
2. `a --help`
3. `a testcases --help`
4. `a testcases generate --help`
5. Reconnect / restore the key.

**Expected**
- All three help pages render immediately with no error and no delay. Help never triggers the entitlement
  check.

### 1.2 Entitled organisation proceeds

**Steps**
1. `a testcases list --limit 1`

**Expected**
- One row; footer `showing 1 of N test cases`.

### 1.3 Region flag before and after the subcommand

**Steps**
1. `a -r eu-central-1 testcases list --limit 1 -o json | jq .total`
2. `a testcases list --limit 1 -o json -r eu-central-1 | jq .total`
3. `a testcases list --limit 1 -o json -r us-east-4 | jq .total`

**Expected**
- Steps 1 and 2 print the same number (the eu-central-1 total). Step 3 prints the us-east-4 total. Neither
  equals the us-west-1 total. (Reference organisation on 2026-09-06: 186 / 38 / 2.)

### 1.4 Bad credentials give a "could not verify" error, not "not in your plan"

**Steps**
1. `SAUCE_ACCESS_KEY=definitely-wrong a testcases list`

**Expected**
- Exit code 1.
- The error starts with `could not verify AI authoring entitlement` and ends with
  `the current user has no organisation`.
- It does **not** contain "not included in your Sauce Labs plan".
- **Known gap**: the second half of the message is imprecise. The service answers the user lookup with
  `401 {"detail":"Authorization failed"}` and the shared user client does not check the status, so the
  gate sees an empty user. Record the message as seen; it will be fixed.

### 1.5 No credentials at all

**Steps**
1. `mv ~/.sauce/credentials.yml ~/.sauce/credentials.yml.bak` (skip if you use env vars)
2. `env -u SAUCE_USERNAME -u SAUCE_ACCESS_KEY ./saucectl authoring --disable-usage-metrics testcases list`
3. Restore: `mv ~/.sauce/credentials.yml.bak ~/.sauce/credentials.yml`

**Expected**
- Error `no credentials set; run 'saucectl configure' or set SAUCE_USERNAME and SAUCE_ACCESS_KEY`; exit 1.

### 1.6 Invalid region

**Steps**
1. `a -r mars testcases list`

**Expected**
- `Error: invalid region "mars"; options: us-west-1, us-east-4, eu-central-1`; exit 1.

---

## Part 2 — Author test cases from the terminal

All authoring targets the public demo site `https://www.saucedemo.com`.

### 2.1 Author a test case and watch it happen (`--wait` on a terminal)

**Steps**
1. Run, in a real terminal:
   ```bash
   a testcases generate --name "manual-$I-login" \
     --intent "Open https://www.saucedemo.com, log in with username standard_user and password secret_sauce, and verify the Products heading is visible." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --wait
   ```
2. Watch the output until it finishes (about a minute).
3. **Record** the new test case ID: `export TC=<id printed on the "New test case:" line>`
4. `export ME=$(a testcases get $TC -o json | jq -r .creatorUserId)`

**Expected**
- First lines: `Generation task accepted.`, `Task ID: …`, `Sauce job ID: …`.
- A spinner reading `waiting for the agent...`, then `queued, 0 step(s) so far...`, then
  `in progress, N step(s) so far...` while the agent works.
- As steps are discovered, lines appear such as `* Entering Username` (reasoning titles) and
  `✓ input_text css=[data-test="username"] ← standard_user` (actions). Each line appears exactly once.
- Final lines: `Generation completed. New test case: <id>` and
  `Inspect it with: saucectl authoring testcases get <id> --show-steps`.
- Exit code 0.

### 2.2 Interrupt the wait with Ctrl-C

**Steps**
1. Start a second authoring, this time with an extra tag that Part 10 relies on:
   ```bash
   a testcases generate --name "manual-$I-login-2" \
     --intent "Open https://www.saucedemo.com, log in with username standard_user and password secret_sauce, and verify the Products heading is visible." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --tag manual-$I-only --wait
   ```
2. About ten seconds after "Generation task accepted", press **Ctrl-C once**.
3. **Record** the task ID from the output: `export TASK2=<Task ID>`

**Expected**
- `Waiting for any in-progress actions to stop... (press Ctrl-c again to exit without waiting)`
- Then: `generation is still running on Sauce Labs. Check progress with: saucectl authoring testcases generate-status <TASK2> --wait`
- Then: `Error: generation is still running on Sauce Labs: context canceled`
- The process exits by itself with code 1; a second Ctrl-C is not needed.

### 2.3 Reattach to the interrupted task

**Steps**
1. `a testcases generate-status $TASK2 --wait`
2. **Record**: `export TC2=<id on the "New test case:" line>`

**Expected**
- If the task is still running: the same streaming lines as 2.1 for the remaining steps. If it already
  finished: goes straight to completion.
- Ends with `Generation completed. New test case: <TC2>` and the inspect hint; exit 0.

### 2.4 Snapshot of a task without waiting

**Steps**
1. `a testcases generate-status $TASK2`
2. `a testcases generate-status $TASK2 -o json`

**Expected**
- Step 1: `Task <TASK2>: COMPLETED`, then `New test case: <TC2>` and the inspect hint. No steps are listed,
  because the service returns none once a task is complete.
- Step 2: exactly one JSON object with `"status": "COMPLETED"` and `"testCaseId": "<TC2>"`.

### 2.5 Fire-and-forget authoring, then wait for it

**Steps**
1. ```bash
   a testcases generate --name "manual-$I-login-3" \
     --intent "Open https://www.saucedemo.com, log in with username standard_user and password secret_sauce, and verify the Products heading is visible." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I
   ```
2. **Record** `export TASK3=<Task ID>`.
3. Poll `a testcases generate-status $TASK3` every 20 seconds until it reads COMPLETED.
4. **Record** `export TC3=<New test case id>`.

**Expected**
- Step 1 returns immediately with exit 0, printing `Generation task accepted.`, the two IDs, and
  `Check progress with: saucectl authoring testcases generate-status <TASK3> --wait`.
- While running, step 3 prints `Task …: QUEUED` or `IN_PROGRESS` with any steps so far and
  `Still running. Reattach with: …`; finally `COMPLETED` with the new ID.

### 2.6 Intent from a file and JSON output while waiting

**Steps**
1. `printf '  Open https://www.saucedemo.com, log in with username standard_user and password secret_sauce, and verify the Products heading is visible.  \n' > $W/intent.txt`
2. ```bash
   a testcases generate --name "manual-$I-login-json" --intent-file $W/intent.txt \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --wait -o json | tee $W/gen.json
   ```
3. `jq -c '{taskId, status, testCaseId}' $W/gen.json`

**Expected**
- No "Generation task accepted" header and no streamed step lines: the only output is a single JSON
  object, printed when the task finishes.
- Step 3 shows `"status":"COMPLETED"` and a 24-hex `testCaseId`.
- `a testcases get <that id>` shows the intent without the surrounding spaces (trimmed).

### 2.7 Intent from standard input; refusal on a terminal

**Steps**
1. ```bash
   echo "Open https://www.saucedemo.com and verify the login button is visible." | \
     a testcases generate --name "manual-$I-stdin" --intent-file - \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' --tag manual-$I
   ```
2. In the terminal, with nothing piped: `a testcases generate --name x --intent-file - --target browserName=chrome`

**Expected**
- Step 1: accepted (task ID printed), exit 0.
- Step 2: `Error: --intent-file - reads standard input, but standard input is a terminal`; exit 1; no request
  made.

### 2.8 Local wait shorter than the task

**Steps**
1. ```bash
   a testcases generate --name "manual-$I-short-wait" \
     --intent "Open https://www.saucedemo.com and verify the login button is visible." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --wait --wait-timeout 10s
   ```
2. Note the task ID; after a minute run `a testcases generate-status <that task>`.

**Expected**
- After about ten seconds: `generation is still running on Sauce Labs. Check progress with: …` and
  `Error: generation is still running on Sauce Labs: context deadline exceeded`; exit 1.
- Step 2 shows the task completed on its own: the local timeout did not cancel it.

### 2.9 Authoring that cannot succeed

**Steps**
1. ```bash
   a testcases generate --name "manual-$I-impossible" \
     --intent "Open https://www.saucedemo.com and click the button labelled 'Purple Elephant'." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --max-steps 6 --wait
   ```

**Expected — one of two outcomes, record which**
- The task ends FAILED: output ends with `Error: generation failed: <CODE>: <detail>` and exit 1; **or**
- the agent gives up gracefully and saves a case: a `✗` step line appears and the run completes with a new
  test case ID.
- Not acceptable: hanging past the generation timeout, or exit 0 with no test case ID.

### 2.10 Input validation happens before any request

Run each command; each must fail immediately (well under a second) with the message shown.

| Command | Expected error contains |
|---|---|
| `a testcases generate --intent x --target browserName=chrome` | `--name is required` |
| `a testcases generate --name n --target browserName=chrome` | `an intent is required` |
| `a testcases generate --name n --intent x --intent-file f --target browserName=chrome` | `mutually exclusive` |
| `a testcases generate --name n --intent x` | `no target specified` |
| `a testcases generate --name n --intent x --target browserName=chrome --target browserName=firefox` | `exactly one target` |
| `a testcases generate --name n --intent x --target browserName=chrome --generation-timeout 30s` | `between 1m and 1h` |
| `a testcases generate --name n --intent x --target browserName=chrome --max-steps 500` | `between 1 and 200` |
| `a testcases generate --name n --intent x --target browserName=chrome $(printf -- '--tag t%s ' $(seq 1 21))` | `at most 20 tags` |

### 2.11 Unknown task ID

**Steps**
1. `time a testcases generate-status 00000000000000000000000000000000`

**Expected**
- `Error: failed to get generation status: ai authoring service error (HTTP 404) TEST_CASE_GENERATION_TASK_NOT_FOUND: Test case generation task not found.`
- Total time about the same as any other command (well under 5 s): the 404 is not retried.

---

## Part 3 — Inspect test cases

### 3.1 Detail view and steps

**Steps**
1. `a testcases get $TC`
2. `a testcases get $TC --show-steps`
3. `export REV=$(a testcases get $TC -o json | jq -r '.revisions[-1].id')`
4. `a testcases get $TC --revision $REV --show-steps`
5. `a testcases get $TC --revision bogus`

**Expected**
- Step 1: a two-column Property/Value table with ID, Name, Tags (`manual-<I>`), Suite `-`, Created/Updated
  with your username, Test URL `https://www.saucedemo.com/`, Tunnel `-`, Primary Target
  `chrome latest / Windows 11`, Run Targets `-`, Revisions `1`, Revision, Intent, Discovered Intent,
  Description, Steps `5` (or however many the agent recorded). Below the table a `Reasoning:` block with
  the agent's titled paragraphs.
- Step 2: the same, followed by a step table with columns `#`, `Action`, `Result`, `Screenshot`, `Reasoning`.
  Actions read like `input_text css=[data-test="username"] ← standard_user`, `click css=[data-test="login-button"]`,
  `assert css=[data-test="title"] toBeDisplayed`, `finish`. Result is `✔`. Screenshot is a 32-character hex
  identifier, **never** a URL. Reasoning is one short sentence.
- Step 4: identical to step 2.
- Step 5: `Error: test case <TC> has no revision bogus`.

### 3.2 JSON mirrors the text view

**Steps**
1. `a testcases get $TC -o json | jq -c '{id, name, tags, steps: (.revisions[-1].steps | length)}'`

**Expected**
- Valid JSON; the values match what 3.1 showed.

### 3.3 Listing filters

**Steps**
1. `a testcases list --search manual-$I`
2. `a testcases list --tag manual-$I-only -o json | jq -r '.items[].id'`
3. `a testcases list --user-id $ME --start-date ${TODAY}T00:00:00Z -o json | jq .total`
4. `a testcases list --tag Login -o json | jq .total` and `a testcases list --tag login -o json | jq .total`
5. `a testcases list --test-suite-id null -o json | jq .total`

**Expected**
- Step 1: only your `manual-<I>-…` cases, footer `showing N of N test cases`.
- Step 2: exactly `<TC2>`.
- Step 3: at least the number of cases you authored today.
- Step 4: the two counts differ when the organisation has both tags (it does in the reference org). Tags
  are case-sensitive.
- Step 5: the number of test cases that belong to no suite (reference org: 119 of 186).

### 3.4 Pagination and the count-only mode

**Steps**
1. `a testcases list --limit 2 -o json | jq -r '.items[].id'`
2. `a testcases list --skip 2 --limit 2 -o json | jq -r '.items[].id'`
3. `a testcases list --limit 0 -o json`
4. `a testcases list --all -o json | jq '.items | length'`

**Expected**
- Steps 1 and 2 print two IDs each, with no overlap.
- Step 3: `{"items":[],"total":N}`.
- Step 4: equals `N`. A warning about a large listing appears only when `N` exceeds 200 (so probably not
  in the reference org).

### 3.5 Empty result and unknown format

**Steps**
1. `a testcases list --search zzzz-nothing-here`
2. `a testcases list -o yaml`

**Expected**
- Step 1: the single line `No test cases found (total: 0).` and no table.
- Step 2: `Error: unknown output format "yaml"; options: text, json`; exit 1.

### 3.6 Not found is answered in one request

**Steps**
1. `time a testcases get 000000000000000000000000`

**Expected**
- `Error: failed to get test case: ai authoring service error (HTTP 404) TEST_CASE_NOT_FOUND: Test case not found.`
- Wall time comparable to a successful `get` (three requests in total: two for the entitlement gate, one
  lookup). Several seconds would mean the 404 is being retried.

### 3.7 Tags list

**Steps**
1. `a testcases list-tags`
2. `a testcases list-tags -o json | jq 'length'`

**Expected**
- One tag per line, including `manual-<I>` and `manual-<I>-only`; case is preserved (`Login` and `login`
  both appear if the organisation has both). Step 2 prints the count.

### 3.8 A stored empty tunnel name is shown honestly

**Steps**
1. `a testcases get 6a6b903c0405fb400076b2ba | grep Tunnel`

**Expected**
- `Tunnel  "" (empty; cleared automatically on run)`. This colleague's test case stores an empty string;
  reading it is harmless. If the row shows `-`, someone has run the case since and cleared it — record
  "not reproducible".

### 3.9 Rename

**Steps**
1. `a testcases rename $TC "manual-$I-login-renamed"`
2. `a testcases get $TC | grep Name`
3. `a testcases rename $TC "$(printf 'x%.0s' $(seq 1 300))"`

**Expected**
- Step 1: `Renamed test case <TC> to "manual-<I>-login-renamed".`
- Step 2: the new name.
- Step 3: a service error with `INVALID_BODY` naming `name` (record the exact wording); the name is
  unchanged.

---

## Part 4 — Run a test case from the CLI and inspect runs

### 4.1 Run with the stored target

**Steps**
1. `a testcases run $TC --build manual-$I`
2. **Record** `export RUN1=<Run id>`.
3. Wait 40 seconds, then `a testcases get-run $TC $RUN1`

**Expected**
- Step 1: `Run <RUN1> started for test case <TC> (build "manual-<I> - 1").` (the service appends ` - 1`),
  a jobs table with Job, Target `chrome latest / Windows 11`, Status `in progress`, URL
  `https://app.saucelabs.com/tests/<job>`, then `Check on it with: saucectl authoring testcases get-run <TC> <RUN1>`.
  Exit 0 immediately.
- Step 3: Status `passed (1/1)`; the jobs table shows `passed`. Open the URL: it is the job in the dashboard.

### 4.2 Run listing is scoped to the test case

**Steps**
1. `a testcases list-runs $TC`
2. `a testcases list-runs $TC -o json | jq -r '[.items[].testCaseId] | unique'`

**Expected**
- Step 1: a table with Run ID, Build, Jobs, Status, Created; footer `showing 1 of 1 runs`.
- Step 2: exactly `["<TC>"]`. Any other ID here would be a **blocker** (it means the whole organisation's
  runs are being returned).

### 4.3 Explicit target as key=value

**Steps**
1. `a testcases run $TC --target 'browserName=firefox,platformName="Windows 11",browserVersion=latest' --build manual-$I`
2. Wait 40 s; `a testcases get-run $TC <run id> -o json | jq -c '.jobs[0] | {target: .target.capabilities.browserName, success, error}'`

**Expected**
- Step 1 accepted; the jobs table shows `firefox latest / Windows 11`.
- Step 2: `success` is `true` or `false` with an `error` string. Record which: the case was authored on
  Chrome, so a Firefox failure is a service outcome, not a saucectl defect.

### 4.4 Explicit target from a JSON file

**Steps**
1. `printf '{"browserName":"chrome","platformName":"Windows 11","sauce:options":{"screenResolution":"1280x1024"}}' > $W/target.json`
2. `a testcases run $TC --target-json @$W/target.json --build manual-$I -o json | jq -c '.jobs[0].target.capabilities'`

**Expected**
- The capabilities include `"sauce:options":{"screenResolution":"1280x1024", …}` (the service adds `build`
  and `name` inside `sauce:options`).

### 4.5 A job that cannot start is reported as failed

**Steps**
1. `a testcases run $TC --target 'browserName=chrome,browserVersion=999,platformName="Windows 11"' --build manual-$I`
2. Wait 15 s; `a testcases get-run $TC <run id>`

**Expected**
- Status `failed (0/1 passed)`; the jobs table shows Status `failed` and Error `Unable to start your session.`

### 4.6 Run detail enforces its test case

**Steps**
1. `a testcases get-run $TC2 $RUN1`

**Expected**
- `Error: failed to get run: ai authoring service error (HTTP 404) TEST_CASE_RUN_NOT_FOUND: …` — the run
  belongs to `$TC`, and the detail endpoint checks that.

### 4.7 Run listing paging and filters

**Steps**
1. `a testcases list-runs $TC --limit 1 -o json | jq -r '.items[].id'`
2. `a testcases list-runs $TC --skip 1 --limit 1 -o json | jq -r '.items[].id'`
3. `a testcases list-runs $TC --all -o json | jq -c '{total, n: (.items|length), tcs: ([.items[].testCaseId]|unique)}'`
4. `a testcases list-runs $TC --user-id $ME --start-date ${TODAY}T00:00:00Z -o json | jq .total`

**Expected**
- Steps 1 and 2 print different IDs.
- Step 3: `n` equals `total` (4 runs so far) and `tcs` is `["<TC>"]`.
- Step 4: the same total (all runs are yours, today).

### 4.8 Revision path (undeclared service surface)

**Steps**
1. `a testcases run $TC --revision $REV --build manual-$I`

**Expected**
- Either accepted like 4.1, or a clear service error with a code. Record which. A hang or a Go stack trace
  would be a defect.

---

## Part 5 — Test suites

### 5.1 Create a suite with a member

**Steps**
1. `a testsuites create --name manual-$I-suite --tag manual-$I --test-case $TC -o json | tee $W/suite.json`
2. `export SUITE=$(jq -r .id $W/suite.json)`
3. `a testsuites get $SUITE`
4. `a testcases list --test-suite-id $SUITE -o json | jq -r '.items[].id'`
5. `a testcases get $TC | grep Suite`

**Expected**
- Step 1: JSON with a 32-hex `id`. Its `testCaseCount` may read `0`: the count in create/update responses
  lags behind the change. That is expected.
- Step 3: Property table with Name, Tags `manual-<I>`, Test Cases `1`, Team, Created/Updated by you, then
  `List its test cases with: saucectl authoring testcases list --test-suite-id <SUITE>`.
- Step 4: exactly `<TC>`. Step 5: `Suite <SUITE>`.

### 5.2 Rename and retag

**Steps**
1. `a testsuites update $SUITE --name manual-$I-suite-2 --tag a --tag b`
2. `a testsuites get $SUITE`

**Expected**
- Step 1: `Updated test suite "manual-<I>-suite-2" (<SUITE>).`
- Step 2: Name `manual-<I>-suite-2`, Tags `a, b`.

### 5.3 Incremental membership

**Steps**
1. `a testsuites update $SUITE --add-test-case $TC2`
2. `a testcases list --test-suite-id $SUITE -o json | jq -r '.items[].id'`
3. `a testsuites update $SUITE --remove-test-case $TC`
4. `a testcases list --test-suite-id $SUITE -o json | jq -r '.items[].id'`
5. `a testcases get $TC | grep Suite`
6. `a testsuites update $SUITE --add-test-case $TC` (put it back for Part 10)

**Expected**
- Step 2: `<TC>` and `<TC2>`. Step 4: only `<TC2>`. Step 5: `Suite -`. Step 6 succeeds.

### 5.4 Rejected updates

**Steps**
1. `a testsuites update $SUITE --test-case $TC --add-test-case $TC2`
2. `a testsuites update $SUITE`

**Expected**
- Step 1: `Error: testCases cannot be combined with addTestCases or removeTestCases`.
- Step 2: `Error: nothing to update: specify at least one change`.
- Both exit 1 immediately: no request was sent.

### 5.5 A second, empty suite and listing filters

**Steps**
1. `export SUITE2=$(a testsuites create --name manual-$I-suite-b -o json | jq -r .id)`
2. `a testsuites list --id $SUITE --id $SUITE2 -o json | jq -r '.items[].name'`
3. `a testsuites list --search manual-$I -o json | jq .total`
4. `a testsuites list --limit 0 -o json`

**Expected**
- Step 2: exactly the two names. Step 3: `2`. Step 4: `{"items":[],"total":N}` (suites support count-only).

### 5.6 Fire-and-forget suite run

**Steps**
1. `a testsuites run $SUITE --build manual-$I-suiterun`

**Expected**
- `Queued 2 run(s) for test suite <SUITE> under build "manual-<I>-suiterun…".` (record whether the name
  comes back with ` - 1` appended) followed by
  `Results are not followed here; see the Sauce Labs dashboard, or use 'saucectl run' with kind: authoring to wait for them.`
- Exit 0. This started real jobs for both members; they will appear in `list-runs`.

### 5.7 Author straight into a suite

Do this **after** Part 10 (an extra member would change the counts expected there). It is listed here so
it is not forgotten; the checkbox belongs to Part 10.7.

---

## Part 6 — Schedules

### 6.1 Timezone is required and `UTC` is rejected

**Steps**
1. `a schedules create --name manual-$I-sched --cron "0 0 3 1 1 *" --test-suite-id $SUITE --state disabled`
2. `a schedules create --name manual-$I-sched --cron "0 0 3 1 1 *" --test-suite-id $SUITE --state disabled --timezone UTC`

**Expected**
- Step 1: `Error: --timezone is required: an IANA region/city zone such as Europe/Berlin (the service does not accept "UTC")`.
- Step 2: `Error: failed to create schedule: ai authoring service error (HTTP 400) INVALID_BODY: Invalid request body.; settings.timezone: Value must be a valid IANA timezone.`

### 6.2 Create a disabled schedule far in the future

**Steps**
1. ```bash
   a schedules create --name manual-$I-sched --cron "0 0 3 1 1 *" --timezone Europe/Berlin \
     --test-suite-id $SUITE --state disabled --max-runs 1 --build manual-$I -o json | tee $W/sched.json
   ```
2. `export SCHED=$(jq -r .id $W/sched.json)`
3. `a schedules get $SCHED`

**Expected**
- Step 1: JSON with `"stateName":"DISABLED"`, `settings.runningUserId` equal to `$ME`, `settings.maxRuns: 1`,
  `settings.buildName: "manual-<I>"`, `nextRunDate` null.
- Step 3: Property table: State `DISABLED`, Cron `0 0 3 1 1 *`, Timezone `Europe/Berlin`, Max Runs `1`,
  Remaining Runs `1`, Build `manual-<I>`, Suites `<SUITE>`, Next Run `-`.

### 6.3 Enable and disable change only the state

**Steps**
1. `a schedules enable $SCHED`
2. `a schedules get $SCHED -o json | jq -c '{state: .state.stateName, settings}'`
3. `a schedules disable $SCHED`
4. `a schedules get $SCHED -o json | jq -c '{state: .state.stateName, settings}'`

**Expected**
- Step 1: `Schedule "manual-<I>-sched" (<SCHED>) is now ENABLED.` Step 3: `… is now DISABLED.`
- Steps 2 and 4: `settings` identical to 6.2 (cron, timezone, runningUserId, maxRuns, buildName all intact);
  only `state` differs.

### 6.4 A partial update keeps everything it does not mention

**Steps**
1. `a schedules update $SCHED --cron "0 0 4 1 1 *"`
2. `a schedules get $SCHED -o json | jq -c .settings`

**Expected**
- Step 1: `Updated schedule "manual-<I>-sched" (<SCHED>), DISABLED, next run -.`
- Step 2: cron is `0 0 4 1 1 *`; timezone, runningUserId, maxRuns `1` and buildName unchanged.

### 6.5 Unset clears a field explicitly

**Steps**
1. `a schedules update $SCHED --unset maxRuns --unset buildName`
2. `a schedules get $SCHED -o json | jq -c .settings`
3. `a schedules update $SCHED --unset cron`

**Expected**
- Step 2: `maxRuns` and `buildName` are **absent**; cron, timezone and runningUserId remain.
- Step 3: `Error: cannot unset "cron"; options: tunnelName, buildName, startDate, endDate, maxRuns`.

### 6.6 Suite membership of a schedule

**Steps**
1. `a schedules update $SCHED --add-test-suite-id $SUITE2`
2. `a schedules get $SCHED -o json | jq -c .testSuiteIds`
3. `a schedules update $SCHED --remove-test-suite-id $SUITE2`
4. `a schedules update $SCHED --remove-test-suite-id $SUITE`
5. `a schedules update $SCHED --test-suite-id $SUITE --add-test-suite-id $SUITE2`

**Expected**
- Step 2: both suite IDs. Step 3 succeeds and leaves only `<SUITE>`.
- Step 4: `Error: a schedule must keep at least one test suite`.
- Step 5: `Error: --test-suite-id cannot be combined with --add-test-suite-id or --remove-test-suite-id`.

### 6.7 Empty update and invalid state

**Steps**
1. `a schedules update $SCHED`
2. `a schedules create --name x --cron "0 0 3 1 1 *" --timezone Europe/Berlin --test-suite-id $SUITE --state running`

**Expected**
- Step 1: `Error: nothing to update: specify at least one change`.
- Step 2: `Error: invalid --state "running"; options: ENABLED, DISABLED`.

### 6.8 Listing

**Steps**
1. `a schedules list --test-suite-id $SUITE`
2. `a schedules list --id $SCHED -o json | jq .total`
3. `a schedules list --search manual-$I -o json | jq .total`
4. `a schedules list --limit 0 -o json`

**Expected**
- Step 1: a row for `manual-<I>-sched` with State `DISABLED` and Next Run `-`. Steps 2 and 3: `1`.
- Step 4: `{"items":[],"total":N}`.

### 6.9 Bad cron is rejected by the service

**Steps**
1. `a schedules create --name x --cron "not a cron" --timezone Europe/Berlin --test-suite-id $SUITE --state disabled`

**Expected**
- A service `INVALID_BODY` error mentioning `settings.cron`. Record the wording.

---

## Part 7 — Variables

All variables in this part use `team` scope unless stated.

### 7.1 Secret from an environment variable

**Steps**
1. `MY_SECRET=abc a variables create --scope team --name manual_${I}_secret --secret --value-from-env MY_SECRET -o json | tee $W/var.json`
2. `export VAR=$(jq -r .id $W/var.json)`
3. `jq 'has("value")' $W/var.json`

**Expected**
- Step 1: JSON with `"isSecret": true`. Step 3: `false` — a secret's value is never in the output.

### 7.2 Plain values from stdin and from a file

**Steps**
1. `echo x | a variables create --scope team --name manual_${I}_stdin --value-from-file - -o json | jq -r .value`
2. `printf 'y\n' > $W/v.txt; a variables create --scope team --name manual_${I}_file --value-from-file $W/v.txt -o json | jq -r .value`

**Expected**
- Step 1 prints `x`, step 2 prints `y`: exactly one trailing newline is stripped.

### 7.3 Masked prompt for a secret (terminal)

**Steps**
1. `a variables create --scope team --name manual_${I}_prompt --secret`
2. Type a value and press Enter.

**Expected**
- A `Value:` prompt; the characters you type are **not** echoed. Then
  `Created secret variable "manual_<I>_prompt" (<id>) at team scope.`

### 7.4 Plain prompt (terminal)

**Steps**
1. `a variables create --scope team --name manual_${I}_prompt2`
2. Type a value and press Enter.

**Expected**
- A `Value:` prompt with visible input; `Created plain variable … at team scope.`

### 7.5 `--value` with `--secret` warns

**Steps**
1. `a variables create --scope team --name manual_${I}_v --secret --value abc`

**Expected**
- A `WRN A secret passed with --value is visible in shell history and the process list; prefer --value-from-env or --value-from-file.`
  line, then `Created secret variable …`.

### 7.6 A secret is never displayed

**Steps**
1. `a variables get $VAR`
2. `a variables get $VAR -o json | jq 'has("value")'`
3. `a variables list --scope team --search manual_$I`
4. `a variables list --scope team --search manual_$I -o json | jq -c '[.items[] | {name, isSecret, hasValue: has("value")}]'`

**Expected**
- Step 1: Value row shows `<secret>`; the last line reads
  `Pass --expected-last-update "<timestamp>" to update or delete against exactly this version.`
- Step 2: `false`.
- Step 3: the secret rows show `<secret>` in the Value column; the plain ones (`manual_<I>_stdin`, `_file`,
  `_prompt2`) show their values. The Last Update column shows the raw timestamp.
- Step 4: `hasValue` is `false` for every secret and `true` for every plain variable.

### 7.7 Conflict detection on update

**Steps**
1. `export T1=$(a variables get $VAR -o json | jq -r .lastUpdate)`
2. `a variables update $VAR --description "first change"`
3. `a variables update $VAR --description "second change" --expected-last-update "$T1"`
4. `a variables update $VAR --description "third change"`

**Expected**
- Step 2: `Updated secret variable "manual_<I>_secret" (<VAR>); new version <timestamp>.`
- Step 3: `Error: variable <VAR> was changed by someone else since it was read; re-read it with 'saucectl authoring variables get <VAR>' and try again`; exit 1.
- Step 4 (no token, default read-then-write): succeeds.

### 7.8 Toggle secrecy

**Steps**
1. `a variables update $VAR --secret=false`
2. `a variables get $VAR | grep -E 'Secret|Value'`
3. `a variables update $VAR --secret=true`
4. `a variables get $VAR | grep -E 'Secret|Value'`

**Expected**
- Per the API documentation the stored value moves across when secrecy is toggled without a new value.
  Step 2 should show `Secret false` and the value visible; step 4 `Secret true` and `<secret>`. Record the
  actual behaviour if it differs — this path was never exercised live.

### 7.9 Scope and identifier pairing

**Steps**
1. `a variables list --scope testSuite`
2. `a variables create --scope org --test-case-id $TC --name manual_${I}_bad --value v`
3. `a variables list --scope testCase --test-case-id $TC -o json | jq .total`

**Expected**
- Step 1: `Error: scope requires its identifier: scope=testSuite requires a test suite id`.
- Step 2: `Error: scope does not accept an identifier: scope=org does not accept a test suite or test case id`.
- Step 3: `0` (valid request, no variables yet).

### 7.10 Suite- and case-scoped variables

**Steps**
1. `export VSUITE=$(a variables create --scope testSuite --test-suite-id $SUITE --name manual_${I}_s --value 1 -o json | jq -r .id)`
2. `export VCASE=$(a variables create --scope testCase --test-case-id $TC2 --name manual_${I}_c --value 2 -o json | jq -r .id)`
3. `a variables list --scope testSuite --test-suite-id $SUITE -o json | jq -r '.items[].name'`
4. `a variables list --scope testCase --test-case-id $TC2 -o json | jq -r '.items[].name'`

**Expected**
- Steps 3 and 4 print `manual_<I>_s` and `manual_<I>_c` respectively.

### 7.11 Limits and validation

**Steps**
1. `a variables list --limit 0`
2. `a variables list --limit 201`
3. `a variables create --scope team --name "Bad Name" --value v`
4. `a variables create --scope team --name manual_${I}_none < /dev/null`
5. `a variables update $VAR`

**Expected**
- Steps 1 and 2: `Error: --limit must be between 1 and 200 for variables; the service has no count-only mode`.
- Step 3: `Error: invalid --name "Bad Name": use lowercase letters, digits and underscores only`.
- Step 4: `Error: no value given; supply one with --value-from-env NAME, --value-from-file PATH (or - for stdin), or --value`.
- Step 5: `Error: nothing to update: specify --name, --description, --secret or a value source`.

---

## Part 8 — Export a test case to code

### 8.1 Available targets

**Steps**
1. `a testcases list-code-targets $TC`

**Expected**
- One target per line, e.g. `typescript_playwright`, `python_selenium`, `java_selenium` (nine in the
  reference org).

### 8.2 Source to standard output

**Steps**
1. `a testcases code $TC --target typescript_playwright > $W/login.spec.ts`
2. `head -5 $W/login.spec.ts`

**Expected**
- The file is Playwright TypeScript (starts with an `import … from '@playwright/test'`). Nothing else was
  printed to the terminal.

### 8.3 Write to a file, refuse to overwrite, force

**Steps**
1. `a testcases code $TC --target python_selenium -f $W/test_login.py`
2. `a testcases code $TC --target python_selenium -f $W/test_login.py`
3. `a testcases code $TC --target python_selenium -f $W/test_login.py --force`

**Expected**
- Step 1: `Wrote N bytes to <W>/test_login.py (python_selenium).`
- Step 2: `Error: <W>/test_login.py already exists; use --force to overwrite`; exit 1.
- Step 3: written again. The byte count may differ from step 1: exports are generated fresh each time.

### 8.4 Write into a directory with a derived filename

**Steps**
1. `a testcases code $TC --target java_selenium -d $W/out/`
2. `ls $W/out/; grep -m1 'public class' $W/out/*.java`

**Expected**
- Exactly one `.java` file whose name equals the `public class` name in the source (for example
  `SauceDemoLoginTest.java` with `public class SauceDemoLoginTest`).

### 8.5 Unavailable target and missing target

**Steps**
1. `a testcases code $TC --target cobol_thing`
2. `a testcases code $TC < /dev/null`

**Expected**
- Step 1: `Error: target "cobol_thing" is not available for this test case; available: <list>`.
- Step 2: `Error: --target is required; available: <list>`.

### 8.6 Interactive target picker (terminal)

**Steps**
1. `a testcases code $TC`
2. Choose `typescript_playwright` with the arrow keys and Enter.

**Expected**
- A select prompt `Export target:` listing the targets; after choosing, the source is printed.

### 8.7 JSON output

**Steps**
1. `a testcases code $TC --target python_selenium -o json | jq -c '{target, len: (.code|length)}'`

**Expected**
- `{"target":"python_selenium","len":<n>}`.

### 8.8 Export of an empty test case

Only if the organisation has a test case with no steps (`a testcases list -o json | jq -r '.items[] | select((.revisions|length)==0 or (.revisions[-1].steps|length)==0) | .id'`).

**Steps**
1. `a testcases code <that id> --target python_selenium`

**Expected**
- An error saying the test case has no steps to export / is empty; never an empty file. Record "not run" if
  no such case exists.

### 8.9 Shell completion

**Steps**
1. Install completion for your shell as `saucectl completion --help` describes, then in a fresh shell:
2. `saucectl authoring variables list --scope <TAB>`
3. `saucectl authoring schedules create --state <TAB>`
4. `saucectl authoring testcases code $TC --target <TAB>`

**Expected**
- Step 2 offers `org team testSuite testCase`. Step 3 offers `ENABLED DISABLED`.
- Step 4 offers **nothing**. **Known gap**: completion runs no pre-run hooks, so the service client is never
  initialised. Record as seen; it will be fixed.

---

## Part 9 — Artifacts

### 9.1 Download a step screenshot by identifier

**Steps**
1. `export SHOT=$(a testcases get $TC -o json | jq -r '.revisions[-1].steps[0].screenshotUrl')`
2. `export SHOTID=$(echo "$SHOT" | sed -E 's#^[^?]*/([^/?]+)(\?.*)?$#\1#')`
3. `a testcases get $TC --show-steps | grep -c "$SHOTID"`
4. `a download-artifact $SHOTID -f $W/step1.png`
5. Open `$W/step1.png`.

**Expected**
- Step 2 yields a 32-character hex identifier (the last path segment of the signed URL).
- Step 3: `1` — the Screenshot column of step 1 shows exactly that identifier, not the URL.
- Step 4: `Wrote N bytes to <W>/step1.png`; the file is a PNG of the login page.

### 9.2 Download by full URL

**Steps**
1. `a download-artifact "$SHOT" -f $W/step1-again.png` (keep the quotes; the URL contains `&`)
2. `cmp $W/step1.png $W/step1-again.png && echo same`

**Expected**
- Same byte count; `same`.

### 9.3 Destination required; no overwrite without `--force`

**Steps**
1. `a download-artifact $SHOTID`
2. `a download-artifact $SHOTID -f $W/step1.png`
3. `a download-artifact $SHOTID -f $W/step1.png --force`

**Expected**
- Step 1: `Error: a destination is required: use -f/--filename`.
- Step 2: `Error: <W>/step1.png already exists; use --force to overwrite`.
- Step 3: written.

### 9.4 Unknown identifier

**Steps**
1. `time a download-artifact 00000000000000000000000000000000 -f $W/x.bin`

**Expected**
- `… (HTTP 404) FILE_NOT_FOUND …`, answered as quickly as any other command; no file created.

---

## Part 10 — Pipeline runs with `kind: authoring`

Create the configuration file used by most scenarios in this part:

```bash
cat > $W/cfg.yml <<EOF
apiVersion: v1alpha
kind: authoring
sauce:
  region: us-west-1
  concurrency: 2
  metadata:
    build: manual-$I-pipeline
defaults:
  timeout: 5m
suites:
  - name: "login on chrome"
    testCases: [$TC]
    targets:
      - capabilities:
          browserName: chrome
          browserVersion: latest
          platformName: "Windows 11"
artifacts:
  download:
    when: always
    match: ["*.mp4", "log.json"]
    directory: $W/artifacts/
reporters:
  junit:
    enabled: true
    filename: $W/report.xml
EOF
```

### 10.1 Dry run starts nothing

**Steps**
1. `export BEFORE=$(a testcases list-runs $TC --limit 1 -o json | jq .total)`
2. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --dry-run`
3. `a testcases list-runs $TC --limit 1 -o json | jq .total`

**Expected**
- Step 2 prints (after the red advisory block, see 10.16) `Running AI-authored tests in Sauce Labs.` and
  then:
  ```
  The following test cases would have run:
    - login on chrome: manual-<I>-login-renamed (<TC>) on 1 configured target(s)
  ```
  Exit 0.
- Step 3 equals `$BEFORE`: no run was started.

### 10.2 A passing run

**Steps**
1. `./saucectl run --disable-usage-metrics -c $W/cfg.yml; echo "exit=$?"`
2. `grep -c '<testcase' $W/report.xml`
3. `find $W/artifacts -type f`

**Expected**
- Log lines: `Starting AI-authored test runs. concurrency=2 testCases=1`, `Run started. … url=https://app.saucelabs.com/tests/<job>`,
  `Runs in progress: 1` every ten seconds, then `Run finished.`
- A **Results** table with one ✔ row: Name `login on chrome - manual-<I>-login-renamed`, Status `passed`,
  Browser `chrome latest`, Platform `Windows 11`, Attempts `1`; footer `All suites have passed`; a
  `Build Link:` that opens the build in the dashboard.
- `exit=0`. Step 2: `1`. Step 3: `<W>/artifacts/login_on_chrome_-_manual-<I>-login-renamed/video.mp4` and `log.json`.

### 10.3 A failing run fails the pipeline

**Steps**
1. `sed 's/browserVersion: latest/browserVersion: "999"/' $W/cfg.yml > $W/fail.yml`
2. `./saucectl run --disable-usage-metrics -c $W/fail.yml; echo "exit=$?"`
3. `grep -E '<failure|failures=' $W/report.xml`

**Expected**
- `ERR Run finished with failures.`; the row shows ✖ and Status `failed`; footer `1 of 1 suites have failed (100%)`;
  `Build Link: N/A` (the job never existed on the Sauce side); `exit=1`.
- Step 3: `<failure message="Unable to start your session." …>` and `failures="1"`.

### 10.4 One result row per target

**Steps**
1. Edit `$W/cfg.yml`: add a second target to the suite:
   ```yaml
       targets:
         - capabilities: {browserName: chrome, browserVersion: latest, platformName: "Windows 11"}
         - capabilities: {browserName: firefox, browserVersion: latest, platformName: "Windows 11"}
   ```
2. `./saucectl run --disable-usage-metrics -c $W/cfg.yml; echo "exit=$?"`
3. `grep -c '<testcase' $W/report.xml`
4. Restore the single-target file afterwards (rerun the `cat > $W/cfg.yml` block).

**Expected**
- Two rows for the one test case, Browser `chrome latest` and `firefox latest`. Step 3: `2`. Exit 0 if both
  passed, 1 if Firefox failed (record which; see 4.3).

### 10.5 Suite referenced by exact name

**Steps**
1. Write `$W/byname.yml` like `cfg.yml` but with the suite block
   ```yaml
   suites:
     - name: "whole suite"
       testSuiteName: manual-<I>-suite-2      # the current name after 5.2
   ```
   (no `targets`, so each case's stored target applies).
2. `./saucectl run --disable-usage-metrics -c $W/byname.yml --dry-run`
3. Change `testSuiteName` to `manual` and dry-run again.
4. `export SUITE3=$(a testsuites create --name manual-$I-suite-2 -o json | jq -r .id)` (a second suite with
   the exact same name), then dry-run the original file again.
5. `a testsuites delete $SUITE3 --yes`

**Expected**
- Step 2 lists both members (`<TC>` and `<TC2>`) `on stored run targets`.
- Step 3: an `ERR failed to execute run command` line whose error reads
  `running AI-authored tests: suite "whole suite": no test suite is named "manual" (the match is exact and case-sensitive)`;
  exit 1. The service's own search is a substring match, so this proves the exact match is enforced
  client-side. (`saucectl run` reports errors as an ERR log line, not as `Error: …`.)
- Step 4: the same kind of line with
  `2 test suites are named "manual-<I>-suite-2"; reference one by testSuiteId instead: <ids>`; exit 1.
- Step 5: `Deleted test suite …`.

### 10.6 Suite referenced by ID, narrowed by tag

**Steps**
1. Write `$W/byid.yml` with
   ```yaml
   suites:
     - name: "tagged only"
       testSuiteId: <SUITE>
       tags: [manual-<I>-only]
   ```
2. `./saucectl run --disable-usage-metrics -c $W/byid.yml --dry-run`
3. Remove the `tags` line and dry-run again.

**Expected**
- Step 2 lists only `<TC2>` (the case authored in 2.2 with the extra tag). Step 3 lists `<TC>` and `<TC2>`.

### 10.7 Author straight into a suite (deferred from 5.7)

**Steps**
1. ```bash
   a testcases generate --name "manual-$I-insuite" \
     --intent "Open https://www.saucedemo.com and verify the login button is visible." \
     --test-url https://www.saucedemo.com \
     --target 'browserName=chrome,platformName="Windows 11",browserVersion=latest' \
     --tag manual-$I --test-suite-id $SUITE --wait
   ```
2. `a testcases get <new id> | grep Suite`

**Expected**
- Step 2: `Suite <SUITE>`.

### 10.8 Concurrency limit

**Steps**
1. Write `$W/ccy.yml` with a suite listing three cases: `testCases: [<TC>, <TC2>, <TC3>]` and no targets.
2. `./saucectl run --disable-usage-metrics -c $W/ccy.yml --ccy 1 2>&1 | grep -E 'Run (started|finished)'`

**Expected**
- The `Run started` lines are separated by roughly one run's duration (~25 s): the second starts only after
  the first finishes. With `--ccy 3` instead, all three start within a second or two.

### 10.9 Async

**Steps**
1. `rm -f $W/report.xml; ./saucectl run --disable-usage-metrics -c $W/cfg.yml --async; echo "exit=$?"`
2. `ls $W/report.xml`

**Expected**
- `Run started (async). Check it with: saucectl authoring testcases get-run <TC> <run>`; the row shows Status
  `in progress`; footer `All suites have launched`; `exit=0`.
- Step 2: no report file (JUnit and JSON reporters are disabled for async runs).

### 10.10 JSON reporter

**Steps**
1. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --reporters.json.enabled --reporters.json.filename $W/report.json`
2. `jq 'length' $W/report.json`

**Expected**
- Step 2: `1` (one entry per job).

### 10.11 Artifact rules: `when: fail` and cleanup

**Steps**
1. `sed 's/when: always/when: fail/' $W/cfg.yml > $W/onfail.yml; rm -rf $W/artifacts`
2. `./saucectl run --disable-usage-metrics -c $W/onfail.yml; find $W/artifacts -type f | wc -l`
3. `mkdir -p $W/artifacts && touch $W/artifacts/old.txt`
4. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --artifacts.cleanup; ls $W/artifacts`

**Expected**
- Step 2: `0` files — a passing run downloads nothing under `when: fail`.
- Step 4: `old.txt` is gone; only the new run's folder exists.

### 10.12 Timeout

**Steps**
1. `sed 's/timeout: 5m/timeout: 5s/' $W/cfg.yml > $W/timeout.yml`
2. `./saucectl run --disable-usage-metrics -c $W/timeout.yml; echo "exit=$?"`

**Expected**
- `ERR Timed out waiting; the run may still be going on Sauce Labs. Check it with: saucectl authoring testcases get-run <TC> <run>`
- The row shows ✖ with Status `?`; footer `1 of 1 suites have failed (100%)`; `exit=1`.
- `a testcases get-run $TC <run>` a minute later shows the run finished on its own.

### 10.13 Ctrl-C during a run

**Steps**
1. `./saucectl run --disable-usage-metrics -c $W/cfg.yml`
2. As soon as `Run started` appears, press **Ctrl-C once**.
3. A minute later: `a testcases get-run $TC <run id from the log>`

**Expected**
- `WRN Interrupted locally; the run continues on Sauce Labs. Check it with: saucectl authoring testcases get-run <TC> <run>`
- The results table still renders, the row `in progress`; exit 1.
- Step 3: `passed (1/1)` — the interruption did not stop the remote run.
- Variant: press Ctrl-C **before** `Run started` appears (during the entitlement check). Expected log:
  `WRN Run was not started: interrupted.`

### 10.14 Unsupported settings are warned about, never silently ignored

**Steps**
1. ```bash
   ./saucectl run --disable-usage-metrics -c $W/cfg.yml --dry-run \
     --retries 2 --tags a --env X=1 --launch-order "fail rate" --show-console-log --live-logs --fail-fast 2>&1 | grep WRN
   ```

**Expected**
- Seven WRN lines about settings, one each for `sauce.retries`, `sauce.metadata.tags`, `env / --env`,
  `showConsoleLog / --show-console-log`, `--live-logs`, `sauce.launchOrder / --launch-order` and
  `--fail-fast`, each saying it is not supported for kind: authoring and will be ignored. (An eighth WRN
  about a newer saucectl version may also appear; ignore it.) The dry run then proceeds.

### 10.15 Select a suite

**Steps**
1. `./saucectl run --disable-usage-metrics -c $W/byname.yml --select-suite "whole suite" --dry-run`
2. `./saucectl run --disable-usage-metrics -c $W/byname.yml --select-suite nope --dry-run`

**Expected**
- Step 1 lists the suite's cases. Step 2: an `ERR failed to execute run command` line with
  `error="no suite named 'nope' found"`; exit 1.

### 10.16 The advisory schema line, before and after merge

**Steps**
1. Before merge: run any `./saucectl run … --dry-run` from this part and look at the first lines.
2. After the PR merges to `main`: rebuild from `main` and repeat.

**Expected**
- Before merge: a red block `There is 1 validation error found in <cfg>:` /
  `- value must be one of "apitest", "cypress", … in /kind`. The run still proceeds (validation is advisory
  and reads the bundle published from `main`).
- After merge: the block is gone.

### 10.17 Build name grouping and truncation

**Steps**
1. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --build manual-$I-grouping`
2. Open the Build Link.
3. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --dry-run --build "$(printf 'b%.0s' $(seq 1 120))" 2>&1 | grep -i 'build name'`

**Expected**
- Step 2: the build is named `manual-<I>-grouping - 1` (the service appends ` - 1`) and contains the job.
- Step 3: `WRN Build name exceeds 100 characters and will be truncated for AI authoring runs.`

### 10.18 Configuration validation

For each file, `./saucectl run --disable-usage-metrics -c <file> --dry-run` must fail immediately, before
any request is made, with an `ERR failed to execute run command` line whose `error=` text is the message
shown, and exit 1.

| Change to `cfg.yml` | Expected error |
|---|---|
| Delete the whole `suites:` block | `no suites configured: add at least one entry under 'suites'` |
| Remove `name:` from the suite | `every suite needs a name` |
| Add `testSuiteId: abc` next to `testCases` | `suite "login on chrome" must set exactly one of testSuiteId, testSuiteName or testCases` |
| Remove `testCases` (keep the name) | same message as above |
| `testCases: [""]` | `suite "login on chrome" has an empty test case id` |
| Replace the target with `- capabilities: {}` | `suite "login on chrome" target 1 has no capabilities` |
| Duplicate the suite block (same name twice) | `suite name "login on chrome" is used more than once` |
| Delete `region: us-west-1` | `no sauce region set` |

---

## Part 11 — Deleting shared assets: the confirmation matrix

Every delete command behaves the same way. Run all four conditions for each asset type; the assets are the
ones you created, so this doubles as teardown. Answer **N** where told, so nothing is deleted before its row.

### 11.1 Schedule

**Steps**
1. Terminal, no `--yes`: `a schedules delete $SCHED` → answer **N**.
2. `a schedules get $SCHED -o json | jq -r .name` (still there).
3. Pipeline simulation: `a schedules delete $SCHED < /dev/null; echo "exit=$?"`
4. Pipeline with bypass: `a schedules delete $SCHED --yes; echo "exit=$?"`

**Expected**
- Step 1: `About to delete schedule "manual-<I>-sched" (<SCHED>).`, then lines
  `- it is DISABLED and runs "0 0 4 1 1 *" (Europe/Berlin)` and `- it triggers 1 suite(s): <SUITE>`, then
  `? Proceed? (y/N)`. Answering N prints `Error: aborted`, exit 1.
- Step 2: the name — nothing was deleted.
- Step 3: `Error: refusing to proceed without confirmation: not running interactively; re-run with --yes to confirm (schedule "manual-<I>-sched" (<SCHED>))`, `exit=1`.
- Step 4: `Deleted schedule "manual-<I>-sched" (<SCHED>).`, `exit=0`.

### 11.2 Test suites

**Steps**
1. Terminal: `a testsuites delete $SUITE` → **N**.
2. Terminal: `a testsuites delete $SUITE --delete-test-cases` → **N**.
3. `a testsuites delete $SUITE < /dev/null; echo "exit=$?"`
4. `a testsuites delete $SUITE --yes; echo "exit=$?"`
5. `a testcases get $TC2 | grep Suite` and `a testcases get $TC | grep Suite`
6. Terminal, accept this time: `a testsuites delete $SUITE2` → **Y**.
7. `a variables get $VSUITE` (the suite-scoped variable from 7.10)

**Expected**
- Step 1: `About to delete test suite "manual-<I>-suite-2" (<SUITE>).` with
  `- 3 test case(s) in the suite will be kept and become unassigned` (TC, TC2 and the 10.7 case). No
  schedule line any more (11.1 deleted it). N → `Error: aborted`.
- Step 2: the same prompt but `- 3 test case(s) in the suite will be DELETED`. N → aborted. Verify the cases
  still exist.
- Step 3: refusal, `exit=1`. Step 4: `Deleted test suite …`, `exit=0`.
- Step 5: both show `Suite -` — the members were kept.
- Step 6: prompt, then `Deleted test suite "manual-<I>-suite-b" (<SUITE2>).`
- Step 7: record whether the suite-scoped variable still exists after its suite is gone (either is
  acceptable; it was never observed). If it exists, delete it with `--yes`.

### 11.3 Variables

**Steps**
1. Terminal: `a variables delete $VAR` → **N**.
2. `a variables delete $VAR --yes --expected-last-update 2026-01-01T00:00:00.000Z; echo "exit=$?"`
3. `a variables delete $VAR < /dev/null; echo "exit=$?"`
4. `a variables delete $VAR --yes; echo "exit=$?"`
5. Terminal, accept: `a variables delete $VCASE` → **Y**.
6. Delete the rest with `--yes`: `manual_<I>_stdin`, `_file`, `_prompt`, `_prompt2`, `_v` (find IDs with
   `a variables list --scope team --search manual_$I -o json | jq -r '.items[].id'`).

**Expected**
- Step 1: `About to delete variable "manual_<I>_secret" (<VAR>).`, `- it is a secret variable at team scope`,
  `- every test referencing {{team:manual_<I>_secret}} will lose it`, `? Proceed? (y/N)`; N → aborted.
- Step 2: the stale-token conflict message (`was changed by someone else since it was read; re-read it with …`),
  `exit=1`, variable still present.
- Step 3: refusal, `exit=1`. Step 4: `Deleted variable …`, `exit=0`.
- Step 5: the prompt names `{{testCase:manual_<I>_c}}` and the test case; Y deletes.
- Step 6: each prints `Deleted variable …`.

### 11.4 Test cases

**Steps**
1. Terminal: `a testcases delete $TC` → **N**.
2. `a testcases delete $TC < /dev/null; echo "exit=$?"`
3. `a testcases delete $TC --yes; echo "exit=$?"`
4. `a testcases list-runs $TC --limit 1 -o json | jq .total`
5. Terminal, accept: `a testcases delete $TC2` → **Y**.
6. Delete every remaining `manual-<I>-…` case with `--yes`:
   `a testcases list --search manual-$I -o json | jq -r '.items[].id'` and loop.

**Expected**
- Step 1: `About to delete test case "manual-<I>-login-renamed" (<TC>).` with
  `- its N recorded run(s) stay in run history but will belong to a test case that no longer exists`
  (N = every run of `<TC>` from Parts 4, 5 and 10); `? Proceed? (y/N)`; N → aborted.
- Step 2: refusal, `exit=1`. Step 3: `Deleted test case …`, `exit=0`.
- Step 4: the run count is unchanged — run history outlives its test case.
- Steps 5 and 6: deleted.

### 11.5 Nothing left behind

**Steps**
1. `a testcases list --search manual-$I -o json | jq .total`
2. `a testsuites list --search manual-$I -o json | jq .total`
3. `a schedules list --search manual-$I -o json | jq .total`
4. `a variables list --scope team --search manual_$I -o json | jq .total`
5. `a testcases list-tags | grep -c manual-$I`
6. `rm -rf $W`

**Expected**
- Steps 1 to 5 all print `0`.

---

## Part 12 — Regression on surfaces the PR touched but did not add

### 12.1 Root help lists the new group once

**Steps**
1. `./saucectl --help | grep -c '^  authoring'`

**Expected**
- `1`.

### 12.2 Other kinds still dispatch and validate

**Steps**
1. `./saucectl run --disable-usage-metrics -c .sauce/playwright.yml --dry-run 2>&1 | tail -5`
2. `./saucectl run --disable-usage-metrics -c .sauce/cypress-10.yml --dry-run 2>&1 | grep -c 'validation error'`
3. After the PR merges, rebuild from `main` and repeat both.

**Expected**
- Step 1 runs the Playwright path (bundling and dry-run output), not `unknown framework configuration`.
- Step 2: `0` both before and after merge — the regenerated bundle did not change validation for existing
  kinds.

### 12.3 Unrelated groups unaffected

**Steps**
1. `./saucectl storage list --limit 1`
2. `./saucectl builds list vdc --size 1`
3. `./saucectl devices list | head -3`

**Expected**
- Each behaves exactly as it does on `main`.

### 12.4 The schema bundle is reproducible

**Steps**
1. `make schema && git status --short api/`

**Expected**
- No output from `git status`: regenerating produces a byte-identical bundle.

### 12.5 CI stamps the version

**Steps**
1. Open the PR's `build` job in GitHub Actions and search the log for `Running version`.

**Expected**
- `Running version v0.0.0+<sha>`, not `0.0.0+unknown`.

### 12.6 Every command has examples

**Steps**
```bash
for g in testcases testsuites schedules variables; do
  for c in $(./saucectl authoring $g --help | sed -n '/^Available Commands:/,/^$/p' | awk '/^  [a-z]/{print $1}'); do
    ./saucectl authoring $g $c --help | grep -q 'Examples:' || echo "missing: $g $c"
  done
done
./saucectl authoring download-artifact --help | grep -q 'Examples:' || echo "missing: download-artifact"
```

**Expected**
- Nothing printed.

### 12.7 Aliases

**Steps**
1. `a tc ls --limit 1 -o json | jq .total`
2. `a ts ls --limit 1 -o json | jq .total`
3. `a schedule ls --limit 1 -o json | jq .total`
4. `a var ls --scope team --limit 1 -o json | jq .total`
5. `a artifact --help | head -1`

**Expected**
- Each resolves to the corresponding command (`tc`=testcases, `ts`=testsuites, `schedule`=schedules,
  `var`=variables, `artifact`=download-artifact, `ls`=list).

---

## Part 13 — Environment-dependent scenarios

Run each if you have the environment; otherwise record **not run** with the reason.

### 13.1 Organisation without the entitlement

**Steps**
1. With credentials of an organisation that does **not** have AI authoring: `a testcases list`
2. `./saucectl run --disable-usage-metrics -c $W/cfg.yml --dry-run` with the same credentials.

**Expected**
- Both: `Error: AI Test Authoring is not included in your Sauce Labs plan; contact your Sauce Labs account team to enable it`;
  no mention of credentials; exit 1.

### 13.2 Sauce Connect tunnel

**Steps**
1. Start a tunnel named `manual-$I-tunnel`.
2. `a testcases run $TC --tunnel-name manual-$I-tunnel --build manual-$I`
3. `a testcases run $TC --tunnel-name no-such-tunnel-$I`
4. Add `sauce.tunnel.name: manual-<I>-tunnel` to `cfg.yml` and run the pipeline; then change it to a
   made-up name and run again.

**Expected**
- Step 2 accepted; the job runs through the tunnel. Step 3: `SC_TUNNEL_NOT_FOUND` with the service's detail.
- Step 4: with the live tunnel, `Performing tunnel readiness check...` then `Tunnel is ready!` then the
  run; with the made-up name the readiness check fails after the tunnel timeout **before** any run starts.

### 13.3 A case that stores an empty tunnel name

Only with the owner's agreement (test case `6a6b903c0405fb400076b2ba` is a colleague's Android emulator
test).

**Steps**
1. `a testcases get 6a6b903c0405fb400076b2ba | grep Tunnel` (must show the empty-string row)
2. `a testcases run 6a6b903c0405fb400076b2ba --build manual-$I` and poll `get-run`.

**Expected**
- The run starts and completes: the runner sends `scTunnelName: null`, which the service accepts. Note that
  this also clears the stored empty string (3.8 will read `-` afterwards).

### 13.4 Real device

**Steps**
1. Author a case with a real-device target (for example `--target-json` with `appium:deviceName` of a real
   device and `platformName=Android`; how the service marks a target as real-device is not documented —
   record what works).
2. Run it through the pipeline.

**Expected**
- Its row appears in a separate real-device results table with its own Build Link.

### 13.5 Another data centre end to end

**Steps**
1. Author a case with `a -r eu-central-1 testcases generate …`.
2. Pipeline config with `sauce.region: eu-central-1` and that case.

**Expected**
- Job URLs use `app.eu-central-1.saucelabs.com`; the run passes.

### 13.6 `make schema` on macOS

**Steps**
1. On a Mac: `make schema`

**Expected**
- Regenerates without a `pushd: not found` error (the Makefile now uses `cd`).

### 13.7 Updating a schedule in an observed state

Only if you own a schedule whose state is `ERRORED` or `RUNNING`.

**Steps**
1. `a schedules update <it> --cron "0 0 5 1 1 *"`
2. `a schedules update <it> --cron "0 0 5 1 1 *" --state disabled`

**Expected**
- Step 1: `Error: schedule is currently ERRORED; pass --state ENABLED or --state DISABLED to update it`.
- Step 2: succeeds.

---

## Recording results

For each scenario record: **pass / fail / not run** (with the reason), the command actually typed when it
differed from the plan, and for a failure the full output plus any run or task ID. Severity guide:

- **Blocker**: wrong exit code from a pipeline run; runs of another test case shown for the requested one
  (4.2); a secret value displayed anywhere; a delete that proceeds without confirmation or bypass.
- **Major**: a documented behaviour absent, or a misleading message.
- **Minor**: formatting.

File blockers against PR #1100 before merge. Do not file the two known gaps (1.4 and 8.9); they are
already tracked.
