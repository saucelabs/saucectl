---
name: manual-test-plan
description: Generate a step-by-step manual test plan for a saucectl change — a pull request, branch or commit range — covering every scenario the change can affect, with exact commands and expected output a QA engineer can execute in order. Use this whenever the user asks for manual testing, a QA plan, test scenarios or an acceptance checklist for a PR or feature, asks "what should be tested manually", wants regression coverage for a schema or shared-code change, or wants to know which scenarios a change impacts — even when they do not say "test plan". Also use it to review or extend an existing manual-test-plan.md.
argument-hint: "PR number, branch or commit range, and optionally the spec directory"
user-invocable: true
---

# Manual test plan for a saucectl change

You are producing one Markdown document that a QA engineer will execute from top to bottom against a
built `saucectl` binary and the live Sauce Labs service. Every scenario in it will be run, so every command
must be exact and every expected result must be something the tester can observe: a string, an exit code, a
file, a count. The document earns its keep by being correct; a plan that quotes a message the code does not
print, or asks the tester to do something no command does, costs more than no plan at all.

Read `references/template.md` and `references/saucectl-conventions.md` before writing anything. Read
`references/review-checklist.md` when you reach step 6. The sibling skill
`.claude/skills/saucectl-dev/SKILL.md` covers building, the schema bundle and CI; consult it rather than
restating it.

## Inputs and output

**Input** is a change: a pull request number (`gh pr view <n> --json baseRefName,headRefName,title,body`), a
branch (compare to `main`), or a commit range. If the change has a spec-kit feature directory
(`specs/NNN-*/`), its `spec.md`, `plan.md`, `quickstart.md` and `research.md` are inputs too; `research.md`
holds behaviours of the remote service that were verified by observation and often contradict the API
documentation. If there is neither a diff nor a spec, ask what the change is before doing anything else.

**Output** is `specs/NNN-*/manual-test-plan.md` when a feature directory exists. Otherwise ask where the
document should live before writing it: this repository has no `docs/` directory, and user-facing
documentation belongs in the separate sauce-docs repository, so there is no established home for an
internal test plan yet. If a plan already exists at the destination, read it first and extend it; a
rewrite discards the tester's familiarity with the numbering.

## The process

### 1. Scope from the diff

Resolve the base branch, then classify every changed file:

```bash
gh pr view <n> --json baseRefName,headRefName          # or: base=main, head=<branch>
git diff --name-status <base>...<head>
```

`A` rows are **new surface**: code that did not exist before, where the risk is that the feature does not
do what it claims. `M`, `D` and `R` rows are **shared surface**: code other kinds and commands already rely
on — the `run` dispatch chain, the root command list, `api/*.schema.json`, the Makefile, CI workflows — where
the risk is that something unrelated broke. Every shared-surface file gets at least one regression
scenario. Write the impact table (change, files, user-visible effect, regression risk) before anything
else; it is the reader's map and your own checklist.

If a spec exists, extract its requirement identifiers (FR-xxx, SC-xxx). Step 6 maps scenarios back to them.

### 2. Inventory the real surface

Build the binary and enumerate what actually exists, from the binary rather than from the spec:

```bash
make build
./saucectl <group> --help
./saucectl <group> <command> --help          # for every command: flags, defaults, aliases
```

Then read the message strings in the code, because the plan quotes them verbatim:

```bash
grep -rn 'Errorf\|errors.New\|Msg(\|Msgf(\|Printf(' internal/cmd/<group>/ internal/<pkg>/
```

A message quoted from memory is the most common way a test plan lies. Copy the string from the source, and
note whether it is printed via cobra (`Error: …`), via zerolog (`ERR … error="…"`) or via `fmt`.

### 3. Learn the facts about the remote service

Read `research.md` and `quickstart.md` if they exist. Whenever a behaviour you want to assert is still a
"should", settle it with a **read-only** probe before it goes in the plan: `--help`, a listing with
`--limit 0`, a `get` of an existing resource, a `--dry-run`, or a `curl` GET with the credentials from
`~/.sauce/credentials.yml` read into a shell variable. Never create, change or delete anything while writing
the plan. The plan is what will do that, with the tester watching, on assets the tester owns.

### 4. Design the sequence

Order the document so that every asset exists before the first scenario that uses it and is deleted after
the last one. In practice:

- **Part 0, setup**: build, a credentials sanity check, `export` statements for every identifier the tester
  will record along the way, a naming convention for throwaway assets (`manual-<initials>-…`), and a helper
  function for a repeated command prefix if there is one.
- **Creation parts** before **inspection parts** before **mutation parts**.
- **Deletion last**, written as the confirmation-matrix scenarios (interactive decline, interactive accept,
  non-interactive refusal, non-interactive bypass). This makes teardown part of the plan rather than an
  afterthought, and it is where destructive-action safeguards get verified.
- **Environment-dependent scenarios** (a second organisation, a Sauce Connect tunnel, real devices, another
  operating system) in their own final part, each saying "record *not run* with the reason if unavailable".
- A **nothing-left-behind** check: searches by the naming convention that must all return zero.

When a later scenario renames or removes something an earlier one created, say so where it happens and add
a "sequencing that matters" note up front. Sequencing bugs are the second most common defect in a plan,
after misquoted messages.

### 5. Write the scenarios

Use the block format from the template: `### N.M Title`, a numbered **Steps** list of exact commands with
`**Record**` lines for values the tester must export, and an **Expected** list of observable results. Use
the table variant only for validation sweeps where each row is one command and one expected message.

Every scenario is to be executed. Do not add coverage, priority or "already verified" columns. The reader
is the tester, and anything that looks like permission to skip will be read as permission to skip. If a
path is already unit-tested, that is a reason to keep its scenario short, not to leave it out: the manual
pass exists precisely to catch what the fakes could not.

Record real defects you discover along the way in a **Known gaps** section, describing the current
behaviour as the expected result, so the tester recognises them and does not file them again.

### 6. Review loop

Follow `references/review-checklist.md`. Re-read the entire document asking three questions: what is stated
that is wrong; what scenario assumes a command, flag, field or behaviour that does not exist; what is
missing. Then check sequencing, placeholder definitions and arithmetic. Fix everything, and start the pass
again from the top. Stop only when a complete pass finds nothing. Several passes is normal; report how many
you ran and what the last one found.

### 7. Deliver

Show `git status --short` and the proposed commit message, then ask whether to commit and push, and stop
until answered. Stage explicit paths only; the working tree routinely holds untracked files that must not
be committed. Commit summaries are capitalised, imperative and at most 50 characters, with the reasoning in
the body.

## Example

Input (from the diff and `--help`): a new `variables delete <id>` command that prompts for confirmation,
refuses when stdin is not a terminal, and accepts `--yes` and `--expected-last-update`.

Output:

```markdown
### 7.13 Delete with tokens
**Steps**
1. `a variables delete $VAR --yes --expected-last-update 2026-01-01T00:00:00.000Z; echo "exit=$?"`
2. `a variables delete $VAR --yes; echo "exit=$?"`
**Expected**
- Step 1: `Error: variable <VAR> was changed by someone else since it was read; re-read it with 'saucectl authoring variables get <VAR>' and try again`; `exit=1`; the variable still exists.
- Step 2: `Deleted variable "manual_<I>_secret" (<VAR>).`; `exit=0`.
```

Both quoted messages were copied from `internal/cmd/authoring/variables_delete.go`; the stale-token
behaviour was confirmed with a read-only probe before the scenario was written.
