# Review checklist for a manual test plan

Run this loop after the first complete draft and after every round of fixes. A pass is one read of the
whole document, top to bottom, applying every check below. Stop only when a pass finds nothing. Writing the
first plan this way took several passes; expect the same, and report in your summary how many passes ran
and what the last one found.

## 1. Wrong statements

For every quoted message, flag or number, find the evidence or fix the text.

| Claim in the plan | How to verify it |
|---|---|
| A quoted error, warning or success message | `grep -rn "<stable fragment>" internal/ cmd/` finds it. Messages contain format verbs (`%s`, `%q`), so grep a fragment without them. Or an observed run shows it. |
| The form of an error (`Error: …` vs `ERR … error="…"`) | `saucectl run` uses the zerolog form; everything else cobra's. See saucectl-conventions.md. |
| A flag, its default or its aliases | `./saucectl <group> <command> --help` lists it with that default. |
| A count or total (rows, files, `<testcase>` elements) | Recompute from a read-only call now, or derive it from what earlier scenarios create. |
| Timing ("within ~10 s", "about a minute") | Only from an observed run; otherwise say "wait until … then check". |
| A service behaviour ("the service rejects …", "the value moves across …") | Point at the observation in research.md or a probe you made. If neither exists, the Expected item must say "record which" and list acceptable and unacceptable outcomes. |

## 2. Assumed scenarios

For every step that changes state, name the command that performs it. If no such command exists, the
scenario is fiction: replace the step with the real mechanism or delete the scenario. Examples from the
first plan: "tag the test case" (there is no tag-edit command; tags are set at authoring time), "count-only
listing" for an endpoint that rejects `limit=0`, a "five-field cron" the service does not accept.

For every field the plan reads from JSON, confirm the key exists in the type (`grep -n 'json:"<key>"' internal/`).

## 3. Missing scenarios

- Walk the requirement identifiers (FR-xxx, SC-xxx) from the spec; every one maps to at least one scenario.
- Walk the command inventory from step 2 of the skill; every command and every flag appears somewhere.
- Every shared-surface row of the impact table has a regression scenario.
- Every interactive path (prompt, masked input, picker, spinner, Ctrl-C) has a terminal scenario, because
  nothing else can exercise it.
- Every destructive command appears in all four cells of the confirmation matrix.
- Environment-dependent paths (second organisation, tunnel, real device, other OS, other data centre) are
  present, each with "record not run if unavailable".
- The plan starts with setup and ends with a nothing-left-behind check.

## 4. Sequencing

- For every placeholder (`$TC`, `$SUITE`, …), the scenario that records it precedes every scenario that
  uses it; the Legend or setup lists where each one comes from.
- Renames, membership changes and deletes happen after the last scenario that depends on the old state.
  Check names used by later scenarios against renames earlier (`testSuiteName` after a suite rename).
- Deletion order respects dependencies: schedules before the suites they trigger; scoped variables before
  their parent suite or case.
- Anything the reader could not infer is in a "sequencing that matters" note at the top of the part.

## 5. Arithmetic and lists

- Counts in Expected match what earlier scenarios created (members in a suite, runs on a case, files in a
  directory).
- Time estimates add up to the number of authoring sessions and pipeline runs the plan starts.
- The teardown / confirmation-matrix part covers every asset type the plan introduces, including the ones
  created "on the side" (probe suites, duplicate-name suites).

## 6. Presentation

- No coverage, priority or "already verified" columns. Everything is to be executed.
- Every command is copy-pasteable: quoting is right (`"Windows 11"` inside a `--target`, `"$URL"` when the
  value contains `&`), placeholders are shell variables, and the helper function is defined before use.
- Known gaps are listed once, with the scenario that hits each one naming the current behaviour as Expected.

## 7. Loop

Fix everything found, then start again at section 1. When a pass finds nothing, the plan is done. Say so
in the summary, with the pass count and the last pass's findings ("pass 3 found nothing").
