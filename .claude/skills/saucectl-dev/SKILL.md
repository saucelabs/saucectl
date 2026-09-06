---
name: saucectl-dev
description: Build, test, lint, run, and release the saucectl Go CLI, and edit its JSON config schemas. Use this skill whenever you are working in the saucectl repository - building or installing the binary, running go test or golangci-lint, invoking the saucectl CLI locally, bumping a framework or macOS/Windows version in api/*.schema.json, regenerating the bundled schema, or figuring out why the check-schema CI job fails. Consult it even for requests that sound routine ("run the tests", "add cypress 15.19 to the schema", "the check-schema job is failing"), because api/saucectl.schema.json is generated rather than hand-edited, the platformName enum is duplicated across four framework schemas, and a stale bundle fails CI with a byte-for-byte diff.
---

# Working on saucectl

`saucectl` is a Cobra-based Go CLI that orchestrates test runs on Sauce Labs. Entry point is
`cmd/saucectl/saucectl.go`; subcommands live in `internal/cmd/*`. There is no server to start -
you build a binary and invoke subcommands.

## Make targets

Running bare `make` self-documents by grepping `#target: @ description` comments out of the Makefile.

| Command | What it does |
|---|---|
| `make build` | `go build cmd/saucectl/saucectl.go` -> `./saucectl` (gitignored) |
| `make install` | `go install` - lands in `$GOBIN` if set, else `$GOPATH/bin`. Check `go env GOBIN` before hunting for the binary |
| `make test` | `go test ./...` |
| `make coverage` | test + per-function coverage report |
| `make lint` / `make lint-fix` | `golangci-lint run` - **requires golangci-lint v2**; `.golangci.yml` declares `version: "2"` and a v1 binary fails to parse it |
| `make format` | `gofmt -w .` |
| `make schema` | regenerates the schema bundle - see below |

## Local builds are always version 0.0.0+unknown

`internal/version/version.go` defines `Version` and `GitCommit` as placeholder defaults overridden at
link time. `make build` passes no ldflags, so this output is expected, not a bug:

```
saucectl version 0.0.0+unknown
(build unknown-commit-sha)
```

GoReleaser injects the real values on release (`.goreleaser.yml`). To reproduce a versioned build:

```bash
go build -ldflags="\
  -X github.com/saucelabs/saucectl/internal/version.Version=v1.2.3 \
  -X github.com/saucelabs/saucectl/internal/version.GitCommit=$(git rev-parse HEAD)" \
  ./cmd/saucectl
```

The build job in `.github/workflows/test.yml` stamps `github.com/saucelabs/saucectl/cli/version.*`,
but no `cli/` package exists - the real one is `internal/version`. Go ignores `-X` for symbols it
cannot find, so CI's stamping silently does nothing. Copy `.goreleaser.yml`, not CI.

## Running the CLI requires credentials - even for --dry-run

The auth check runs before the dry-run branch, so `--dry-run` is not a way to exercise the CLI
without an account:

```
SauceCTL requires a valid Sauce Labs account!
Error: no credentials set
```

`internal/credentials/credentials.go` resolves in this order, first hit wins:

1. `$SAUCE_USERNAME` / `$SAUCE_ACCESS_KEY`
2. `~/.sauce/credentials.yml` (written by `saucectl configure`)

The repo ships working configs in `.sauce/` (`cypress-10.yml`, `playwright.yml`, `espresso.yml`,
`testcafe.yml`, `replay.yml`, `xctest.yml`, `xcuitest.yml`) with fixtures under `tests/e2e/`:

```bash
./saucectl run -c .sauce/cypress-10.yml --dry-run             # bundle + validate, no jobs
./saucectl run -c .sauce/cypress-10.yml --select-suite "chrome test"
./saucectl run -c .sauce/cypress-10.yml --verbose              # zerolog debug level
```

`--dry-run` builds and validates the upload bundle without launching jobs, which makes it the right
first check after touching packaging or config code. Ctrl-C is graceful - the first press cancels the
context so in-flight jobs clean up, the second exits hard.

## The schema: one generated file, many source files

`api/saucectl.schema.json` is **generated**. Do not hand-edit it. It is bundled from
`api/global.schema.json`, which dispatches on `kind` + `apiVersion` to per-framework schemas that in
turn `$ref` shared subschemas:

```
api/global.schema.json              <- entry point, kind -> framework dispatch
api/v1/framework/                   <- cypress.schema.json only
api/v1alpha/framework/              <- apitest, espresso, playwright, playwright-cucumberjs,
                                       replay, testcafe, xctest, xcuitest (all *.schema.json)
api/*/subschema/                    <- sauce, artifacts, common, npm, reporters
api/saucectl.schema.json            <- GENERATED BUNDLE, never edited by hand
```

### Regenerating

```bash
make schema      # rewrites api/saucectl.schema.json in place
```

Needs Node plus a one-time `npm ci` in `scripts/json-schema-bundler/`. CI pins Node 20 and installs
from the committed `package-lock.json`. Matching that matters, because the bundle is compared
byte-for-byte: the `check-schema` job regenerates and runs a plain `diff` against the committed
file, so a stale bundle fails the build. To check without overwriting, write the fresh copy outside
the repo - `api/fresh.schema.json` (the path CI uses) is **not** gitignored and will otherwise
linger as an untracked duplicate:

```bash
(cd scripts/json-schema-bundler && \
   npm run bundle -- -s ../../api/global.schema.json -o /tmp/fresh.schema.json)
diff api/saucectl.schema.json /tmp/fresh.schema.json   # must be empty
```

### Recipe: bumping a framework or platform version

This is the most common change in the repo - most recent commits are exactly this. Edit the source
schema, then regenerate; the commit should touch both the source file and the bundle, which is why
these diffs always show them together.

**Framework versions** live in the framework schema's `version` enum: `cypress`, `playwright`,
`testcafe`, `playwright-cucumberjs`.

**Platform values (macOS/Windows)** are duplicated across **four** framework schemas, each with its
own `platformName` enum that must be updated together:

| File | Shape |
|---|---|
| `api/v1/framework/cypress.schema.json` | `$ref` to `common.schema.json` + local `enum` narrowing it |
| `api/v1alpha/framework/playwright.schema.json` | same `$ref` + `enum` shape |
| `api/v1alpha/framework/playwright-cucumberjs.schema.json` | same `$ref` + `enum` shape |
| `api/v1alpha/framework/testcafe.schema.json` | **bare `enum`, no `$ref`** - do not add one |

Two traps worth knowing, because nothing catches either one:

- Missing `playwright-cucumberjs` is easy, and the bundle still regenerates cleanly and CI's `diff`
  still passes - the only symptom is `kind: playwright-cucumberjs` configs rejecting a platform every
  other framework accepts.
- `testcafe.schema.json` contains a **second** `platformName` further down, the iOS simulator enum
  (`"enum": ["iOS"]`). Match on the surrounding block, not the first hit.

Steps: add the value to every relevant source enum, keeping existing ordering (oldest to newest) so
diffs stay readable; regenerate with `make schema`; confirm the diff covers both the
source schemas and `api/saucectl.schema.json`.

## Matching CI before you push

`.github/workflows/test.yml` runs four gates:

```bash
make lint                # golangci-lint v2
make test                # go test ./...
make build               # build
make schema              # check-schema: then confirm the bundle diff is only what you intended
```

Regeneration rewrites the bundle in place, so afterwards it is in sync by construction - the gate is
remembering to run it. If `git status` shows `api/saucectl.schema.json` changing when you only meant
to touch Go code, you had a stale bundle committed. `git diff --exit-code` is *not* the right check:
when you are deliberately changing a schema, that diff is supposed to be non-empty.

`goreleaser check` also runs, pinned to `~> v2.15` on purpose - v2.16 turned the deprecated `brews`
block in `.goreleaser.yml` into a hard error. Leave the pin until the formula-to-cask migration ends.

## Conventions worth honoring

Spelled out in `CONTRIBUTING.md`; the ones that actually come up in review: `gofmt -s` everything;
document all declarations including unexported ones; no `utils`/`helpers` packages; keep variable
name length proportional to scope; write tests with plain `go test` rather than another framework.
Commit summaries are capitalized, imperative, and under ~50 characters.
