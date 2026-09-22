# Template for a saucectl manual test plan

Copy the skeleton, replace every `<…>`, delete parts that do not apply, and keep the order. The example
lines under each heading are taken from `specs/001-ai-test-authoring/manual-test-plan.md`, the first plan
written this way; read that file when you want to see a complete instance.

## Header

```markdown
# Manual Test Plan: <feature or change> (<PR #n | branch | commit range>)

**Feature**: [spec.md](./spec.md) | **Quickstart**: [quickstart.md](./quickstart.md) | **Research**: [research.md](./research.md) | **Date**: <YYYY-MM-DD>
```

Include only the links that exist.

## How to use this plan

Tell the tester, in a short bulleted list:

- Run **every** scenario, **in order**; later parts use assets created in earlier ones.
- Each scenario has numbered **Steps** and an **Expected** list; it passes only when every Expected item
  holds. Record pass / fail / not run per scenario ID.
- Commands use shell variables for recorded values (`$TC`, `$SUITE`, …); whenever a step says **Record**,
  export the value before moving on.
- If a helper function is defined in Part 0 (e.g. `a` for `./saucectl authoring --disable-usage-metrics`),
  say so here.
- Run from a real terminal (not `script`, CI or a pipe); scenarios that simulate a pipeline redirect stdin
  from `/dev/null` explicitly.
- When the change adds a configuration `kind`: until the PR merges, `saucectl run` prints one red advisory
  block ("value must be one of … in /kind"); name the scenario that checks it.
- Point at the **Known gaps** section: behaviours listed there are expected as described and must not be
  filed as new.
- A rough total time, including how many authoring sessions or pipeline runs the plan starts.

## Impact analysis (optional but recommended for feature PRs)

| Change | Files | User-visible impact | Regression risk |
|---|---|---|---|

One row per change area; the shared-surface rows are the ones that justify the regression part.

## Known gaps found while writing this plan

| Gap | Where | Effect | Scenario |
|---|---|---|---|

Only real defects discovered during steps 2–6. Describe the current behaviour; the scenario that exercises
it lists that behaviour as Expected.

## Part 0 — Setup

```markdown
### 0.1 Build the binary from the branch
### 0.2 Credentials and entitlement (a one-line sanity command and its expected footer)
### 0.3 Shell setup
```

Typical shell setup:

```bash
a() { ./saucectl authoring --disable-usage-metrics "$@"; }   # only when one prefix repeats throughout
export I=<your initials, lowercase>
export W=/tmp/<topic>-manual && mkdir -p "$W"
export TODAY=$(date -u +%Y-%m-%d)
```

State the naming rule (`manual-$I-…`; variables `manual_$I_…`) and the rule never to delete, rename or move
an asset the tester did not create.

## Parts 1..N — one per area, in dependency order

Each scenario:

```markdown
### N.M Title

**Steps**
1. `exact command`
2. **Record** `export X=<value printed by step 1>`
3. `exact command using $X`

**Expected**
- Step 1: `verbatim output line or fragment`; exit code.
- Step 3: what the tester sees; a file path; a count.
```

Rules for the block:

- One behaviour per scenario. If Expected needs the words "and also", split it.
- Expected items are observable: quote strings from the source, name exit codes, list files, give counts.
  Avoid "works", "succeeds", "correct".
- Where the outcome genuinely depends on the service (a browser the case was not authored on, a field the
  API documents but nobody has exercised), say "record which" and list the acceptable outcomes and the
  unacceptable ones (hang, silent exit 0, stack trace).
- For validation sweeps use a table: one command per row, one expected message per row, and one sentence
  above it saying what all rows share (e.g. "each fails immediately, before any request, with the message
  shown").

Sequencing that the reader cannot infer goes in a note at the top of the part:

```markdown
Run 6.2 (create a schedule on `$SUITE`) before 5.6 so the delete prompt has a schedule to list.
```

## Part: Deleting shared assets — the confirmation matrix

For each asset type, four scenarios or steps, in this order, on the tester's own assets:

1. Interactive, no `--yes`, answer **N**: the prompt names the asset and what it affects; `Error: aborted`; asset still there.
2. Non-interactive (`< /dev/null`), no `--yes`: refusal message; exit 1.
3. Non-interactive with `--yes`: deleted; exit 0.
4. Interactive, answer **Y** (on the next asset of the same type): deleted.

This part is also the teardown; order asset types so dependants go first (schedules before suites, suite-
or case-scoped variables before their parents).

## Part: Regression on surfaces the PR touched but did not add

One scenario per shared-surface row of the impact table: root help still lists everything once, other
kinds still dispatch and validate, `make schema` reproduces the bundle byte for byte, unrelated command
groups behave as on `main`, CI artefacts (version stamp) as expected.

## Part: Environment-dependent scenarios

Second organisation without the entitlement, Sauce Connect tunnel, real devices, another data centre,
macOS-only build steps. Each begins "Run if you have …; otherwise record **not run** with the reason."

## Nothing left behind

Searches by the naming convention across every asset type, each expected to return `0`, plus `rm -rf $W`.

## Recording results

```markdown
For each scenario record: pass / fail / not run (with the reason), the command actually typed when it
differed from the plan, and for a failure the full output plus any run or task ID. Severity guide:
- Blocker: wrong exit code from a pipeline run; data shown for the wrong resource; a secret displayed;
  a delete that proceeds without confirmation or bypass.
- Major: a documented behaviour absent, or a misleading message.
- Minor: formatting.
File blockers against the PR before merge. Do not file the known gaps; they are already tracked.
```
