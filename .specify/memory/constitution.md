# saucectl Constitution

saucectl is a command-line tool that orchestrates test runs on Sauce Labs. It is the interface between
a customer's pipeline and a remote service it does not control. Its principles follow from that: it must
be idiomatic Go, predictable to someone who already knows the tool, and honest about what the remote
service actually does rather than what its documentation claims.

Sources: `CONTRIBUTING.md` (Coding Style, rules 1–10), `.github/workflows/test.yml` (quality gates), and
the `saucectl-dev` skill (build, schema and release workflow).

## Core Principles

### I. Idiomatic Go, Enforced by Tooling

All code is formatted with `gofmt -s` and passes `go vet` at default levels. It follows Effective Go and
the Go Code Review Comments. `golangci-lint` (**v2** — a v1 binary cannot parse `.golangci.yml`) runs
`errname`, `revive` and `staticcheck`; `goimports` is the configured formatter, grouping stdlib,
third-party, then `github.com/saucelabs/saucectl/...`. Package names carry no underscores. Variable name
length is proportional to scope — short methods take short names, globals take longer ones.

*Rationale*: consistency is not aesthetic here. A contributor navigating an unfamiliar package should
spend their attention on the logic, not on decoding a local dialect.

### II. Document Every Declaration, Including Unexported Ones

Every declaration and method carries a comment — private ones too. Comments state the **why**, the
history and the context, along with expectations and caveats. Documenting an unexported type now means it
is ready the day it gets exported.

*Rationale*: this repository integrates with a service whose behaviour is frequently surprising. A
comment explaining *why* a field is a pointer, or why a retry is disabled, is often the only surviving
record of a hard-won discovery.

### III. No Utils, No Helpers

There is no `utils` or `helpers` package, and none will be created. A function not general enough to
justify its own well-named package is not general enough to be shared: leave it unexported and
well-documented in the package that uses it.

*Rationale*: `utils` packages accumulate unrelated code and become a dependency everything imports and
nobody understands.

### IV. Plain `go test`, No New Frameworks

All tests run under `go test` with no outside tooling. Table-driven tests are the default idiom. An
assertion package is acceptable only where it provides *real* incremental value. Fakes are hand-written;
fakes shared across packages live in `internal/mocks`, fakes used by one package live in its test file.

*Rationale*: a test suite that runs with the standard toolchain stays runnable years later, on any
machine, without archaeology.

### V. The Bundled Schema Is Generated, Never Hand-Edited

`api/saucectl.schema.json` is a build artifact. The sources of truth are `api/global.schema.json`, the
per-framework schemas and the shared subschemas. Regenerate with `make schema` and commit the source
change and the regenerated bundle together. CI compares byte-for-byte, so a stale bundle fails the build.
When comparing without overwriting, write the fresh copy outside the repository — `api/fresh.schema.json`
is not gitignored.

*Rationale*: two sources of truth for the same schema guarantees they will diverge, and the divergence
will surface as a user's config being rejected for no visible reason.

### VI. New Surface Matches Existing Surface

A new command is predictable to someone who already uses the tool. Output format is chosen with
`-o/--out` (`text` or `json`); file destinations use `-f/--filename` or `-d/--target-dir`, never `-o`.
Region comes from `-r/--region`. Tables use the shared style in `internal/tables`. Commands collect usage
metrics the way existing commands do. Domain types and their service interfaces live together in
`internal/<domain>`; the HTTP implementation lives in `internal/http`.

*Rationale*: every departure from precedent is a thing each user must learn separately. Deliberate
departures are permitted, but they are recorded and justified — never accidental.

### VII. Verify Remote Behaviour; Do Not Trust Its Documentation

Behaviour of the Sauce Labs APIs is established by observation against the live service, not by reading a
specification. Where a specification and observed behaviour disagree, observed behaviour wins and the
disagreement is recorded in a comment. Claims about remote behaviour that have not been verified are
labelled as unverified, and carry a verification step.

*Rationale*: this is earned experience. Published specifications for these services have been found to
declare the wrong authentication scheme, omit fields that carry the actionable error text, and document
path parameters that are silently ignored in favour of a query parameter. Code written from the document
rather than the service compiles, passes review, returns plausible data, and is wrong.

### VIII. Fail Loudly, Never Silently

A setting the tool cannot honour produces a warning, not silence. An error surfaces the remote service's
own machine-readable code and human-readable detail. Nothing waits indefinitely: every wait is bounded
and, on expiry, says what is still running and how to resume. A command that cannot prompt because it is
not interactive refuses and explains, rather than hanging or proceeding unconfirmed.

*Rationale*: this tool runs unattended in pipelines. A silent no-op there is discovered weeks later, in
production, by someone who had every reason to believe the test ran.

## Quality Gates

Four gates run in CI (`.github/workflows/test.yml`) and must pass before merge:

| Gate | Command | Note |
|---|---|---|
| Lint | `make lint` | requires golangci-lint **v2** |
| Test | `make test` | `go test ./...` |
| Build | `make build` | |
| Schema | `make schema` | bundle compared byte-for-byte; only run when schema sources change |

`make format` and `make coverage` are available locally. Local builds report `0.0.0+unknown` by design —
GoReleaser injects real version values at release time.

## Development Workflow

Work happens on a fork, in a feature branch named `XXXX-something` after the issue it addresses; any
significant change is announced as an issue first. Unit tests accompany every change. Commit summaries
are capitalized, imperative and at most 50 characters, with detail in the body after a blank line.
Pull requests are cleanly rebased rather than merged, reference the issues they close, and include
documentation changes in the same PR — so that a revert removes every trace of the change.

Anything user-facing is documented in the `saucelabs/sauce-docs` repository; this repository's README
does not document individual commands.

## Governance

This constitution records the conventions this repository already follows; it does not invent new ones.
Where it conflicts with `CONTRIBUTING.md`, `CONTRIBUTING.md` wins and this document is corrected.

Amendments require a rationale and, where behaviour changes, a migration note. Reviews verify compliance;
a deliberate violation must be justified in the pull request and recorded in the plan's Complexity
Tracking section rather than passed over in silence.

These are guidelines held firmly, not laws. Rule 10 of `CONTRIBUTING.md` applies: having read them, you
now know they are guidelines. Apply judgement, and leave the codebase better than you found it.

**Version**: 1.0.0 | **Ratified**: 2026-09-05 | **Last Amended**: 2026-09-05
