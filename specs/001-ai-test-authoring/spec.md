# Feature Specification: AI Test Authoring in saucectl

**Feature Branch**: `001-ai-test-authoring`

**Created**: 2026-09-05

**Status**: Draft

**Input**: User description: "Add Sauce Labs AI Test Authoring support to saucectl: a command group covering the AI Test Authoring service (test cases, test suites, test schedules, variables, artifacts), plus a new config kind so authored suites run via `saucectl run` with the shared reporters, artifact download, concurrency and CI exit codes."

## User Scenarios & Testing *(mandatory)*

Sauce Labs offers an AI authoring capability: a person describes a test in plain language, an AI agent
drives a real browser to work out how to perform it, and the result is saved as a reusable test. Those
tests can be grouped, scheduled, and parameterised with shared values.

Today that capability is reachable only through the web interface and the IDE plugins. The people who
run tests for a living — release engineers and QA engineers working in pipelines — cannot reach it from
the command line, so AI-authored tests cannot gate a build, cannot be managed alongside the code they
test, and cannot participate in the workflows every other test type in saucectl already supports.

### User Story 1 - Gate a pipeline on AI-authored tests (Priority: P1)

A release engineer adds AI-authored tests to a continuous integration pipeline. The pipeline runs the
tests against the browsers and devices the team cares about, waits for the outcome, prints a summary in
the same shape as every other test type, and fails the build when a test fails.

**Why this priority**: This is the entire reason the capability needs to exist in a command-line tool.
Without it, AI-authored tests remain a manual, human-triggered activity and cannot protect a release.
Every other story is a convenience by comparison.

**Independent Test**: Configure one authored suite, run it from a shell, and confirm the process reports
each result and returns a failure status when any test fails and a success status when all pass. Delivers
a working release gate on its own, with no other story implemented.

**Acceptance Scenarios**:

1. **Given** a configured suite of authored tests that all pass, **When** the engineer runs it, **Then**
   each test's outcome is printed with a link to its result and the process reports overall success.
2. **Given** a configured suite where at least one test fails, **When** the engineer runs it, **Then** the
   failure is clearly identified and the process reports overall failure so the pipeline stops.
3. **Given** a suite configured to run against several browsers, **When** it runs, **Then** each
   browser's outcome is reported separately rather than collapsed into a single result.
4. **Given** a run that has been started, **When** the engineer interrupts it, **Then** work already
   started in the cloud is reported as still running and the engineer is told how to check on it.
5. **Given** a team that groups results by build, **When** a suite runs, **Then** every result is
   associated with that build so they can be reviewed together.

---

### User Story 2 - Manage authored tests from the command line (Priority: P2)

A QA engineer inspects, organises and maintains the team's authored tests without opening a browser:
listing and filtering tests, reading what a test actually does, renaming and removing tests, grouping
them, scheduling them, and managing the shared values they depend on.

**Why this priority**: Once tests gate a pipeline, the assets behind that gate must be maintainable from
the same place. This is the bulk of the day-to-day surface, but it has no value until something runs.

**Independent Test**: List the organisation's authored tests, open one, and confirm its recorded steps
are readable. Delivers immediate value as an inspection tool even with no other story implemented.

**Acceptance Scenarios**:

1. **Given** an organisation with many authored tests, **When** the engineer lists them and narrows by
   name, label or grouping, **Then** only matching tests are returned.
2. **Given** a specific authored test, **When** the engineer inspects it, **Then** its name, labels,
   ownership, history and the sequence of actions it performs are shown in readable form.
3. **Given** a specific authored test, **When** the engineer asks for its past results, **Then** only
   that test's results are returned and never another test's.
4. **Given** a group of authored tests, **When** the engineer adds or removes members, **Then** the group
   reflects the change and unrelated members are untouched.
5. **Given** a shared value marked as confidential, **When** the engineer views it, **Then** its content
   is never displayed.
6. **Given** a shared value that a colleague has changed since the engineer last read it, **When** the
   engineer tries to change it, **Then** the conflict is reported rather than silently overwriting the
   colleague's change.
7. **Given** a recurring schedule, **When** the engineer suspends and resumes it, **Then** its state
   changes accordingly and no runs are triggered while suspended.
8. **Given** a shared asset that other work depends on, **When** the engineer asks to remove it, **Then**
   they are shown what will be affected and the removal happens only after they confirm.
9. **Given** an automated job with no one available to answer, **When** a removal is attempted without an
   explicit bypass, **Then** the command refuses and explains, rather than hanging or proceeding.

---

### User Story 3 - Author a test from a description, in the terminal (Priority: P3)

An engineer describes a test in plain language and watches the AI work out how to perform it, seeing each
action as it is attempted, without switching to a browser. When it finishes, the test is saved and ready
to run.

**Why this priority**: This is the most distinctive capability, but authoring is a lower-frequency
activity than running or maintaining, and the web interface already serves it adequately. It becomes
valuable in the terminal mainly when combined with the stories above.

**Independent Test**: Describe a simple journey against a public site, watch the actions stream, and
confirm a runnable test is saved at the end. Delivers value alone as a faster authoring loop.

**Acceptance Scenarios**:

1. **Given** a plain-language description and a starting address, **When** the engineer requests
   authoring and asks to wait, **Then** each action the AI attempts is shown as it happens along with
   whether it succeeded.
2. **Given** authoring that completes, **When** it finishes, **Then** the engineer is told the new test's
   identity and how to inspect it.
3. **Given** authoring that fails, **When** it finishes, **Then** the reason is reported and the process
   returns a failure status.
4. **Given** authoring in progress, **When** the engineer interrupts, **Then** they are told the work
   continues remotely and given the means to reattach to it later.
5. **Given** an engineer who did not ask to wait, **When** authoring is accepted, **Then** they are given
   a reference and told how to check progress later.

---

### User Story 4 - Take an authored test to code (Priority: P4)

An engineer converts an AI-authored test into source code in their team's own testing framework and
language, so it can be reviewed, version-controlled and maintained like any other test — and run through
the tooling the team already uses.

**Why this priority**: A valuable escape hatch that prevents lock-in and lets AI authoring act as a
starting point rather than a destination. It is genuinely optional: teams can use authored tests
indefinitely without ever exporting them.

**Independent Test**: Export one authored test to a chosen language and framework, save it to a file, and
confirm the file is valid source that a developer would recognise. Delivers value alone as a migration
path.

**Acceptance Scenarios**:

1. **Given** an authored test, **When** the engineer asks which languages and frameworks it can be
   exported to, **Then** the available choices are listed.
2. **Given** a chosen language and framework, **When** the engineer exports, **Then** the source is
   written where they asked, with a name and extension appropriate to that language.
3. **Given** a choice that is not available for that test, **When** the engineer exports, **Then** they
   are told which choices are available rather than receiving an opaque failure.
4. **Given** an export target that already exists on disk, **When** the engineer exports, **Then** the
   existing file is not destroyed unless they explicitly ask for it to be replaced.

---

### Edge Cases

- **The organisation does not have the capability.** The engineer must be told plainly that it is not
  part of their plan, distinctly from being told their credentials are wrong or the service is down.
- **Results are requested for one test but belong to another.** Results must always be scoped to the test
  the engineer asked about; showing another test's results is a correctness failure, not a display quirk.
- **A run never reaches a conclusion.** Waiting must be bounded, and on expiry the engineer must learn
  that the work may still be running rather than being told it failed.
- **Two people change the same shared value at once.** The second change must be refused with an
  explanation, not silently applied over the first.
- **A confidential value is needed by a command.** It must be suppliable without ever appearing in shell
  history or in the list of running processes.
- **A test is asked to run with no target.** Either the test's own saved targets apply, or the engineer is
  told none exist — never an unexplained failure.
- **A very large organisation lists everything.** The engineer must not be left waiting indefinitely with
  no indication of volume.
- **A test that records nothing is exported.** The engineer must be told the test is empty rather than
  receiving an empty file.
- **A resource is requested that does not exist.** The engineer must be told promptly, without the tool
  appearing to hang while it retries.
- **A removal is attempted from an automated job with no one to answer.** The command must refuse and
  explain, never block waiting for an answer that cannot come and never proceed unconfirmed.
- **A removal is confirmed for an asset others depend on.** The engineer must have been shown what else
  is affected before confirming, not after.

## Requirements *(mandatory)*

### Functional Requirements

**Running and reporting**

- **FR-001**: Users MUST be able to run one authored test, or every test in a group, from a single command.
- **FR-002**: Users MUST be able to describe a set of authored tests to run in the project's existing
  configuration file, alongside the other test types the tool already supports.
- **FR-003**: The system MUST report a distinct outcome for every browser or device a test ran against.
- **FR-004**: The system MUST return a failure status when any test fails, and a success status only when
  all tests pass, so that pipelines can gate on it.
- **FR-005**: The system MUST present results in the same summary format used by the tool's other test
  types, including a link to each result.
- **FR-006**: The system MUST support the existing machine-readable result reports, and those reports MUST
  contain one entry per test rather than being structurally empty.
- **FR-007**: The system MUST allow results to be grouped under a build name shared by the whole run.
- **FR-008**: The system MUST allow the number of tests running at once to be limited.
- **FR-009**: The system MUST allow a run to be started without waiting for results, for callers that do
  not need to block.
- **FR-010**: The system MUST allow a run to be routed through the organisation's private network
  connection.
- **FR-011**: The system MUST allow a preview that shows exactly which tests would run without running
  them or incurring cost.
- **FR-012**: The system MUST allow files produced by a run to be retrieved automatically, subject to the
  project's existing retrieval settings.
- **FR-013**: Where the tool offers a setting that this capability cannot honour, the system MUST warn
  the user rather than silently ignoring it.

**Managing tests, groups, schedules and shared values**

- **FR-014**: Users MUST be able to list authored tests and narrow the list by name, label, owner, team,
  group and creation date.
- **FR-015**: Users MUST be able to inspect a single authored test, including the sequence of actions it
  performs and the reasoning recorded for each.
- **FR-016**: Users MUST be able to rename and remove authored tests.
- **FR-017**: Users MUST be able to list the labels in use across the organisation.
- **FR-018**: Users MUST be able to review a test's past results, and results returned MUST belong only
  to the test requested.
- **FR-019**: Users MUST be able to create, inspect, change and remove groups of tests, including adding
  and removing individual members without restating the whole group.
- **FR-020**: Users MUST be able to create, inspect, change, suspend, resume and remove recurring
  schedules, including when they start, when they stop, and how many times they may run.
- **FR-021**: Users MUST be able to create, inspect, change and remove shared values at organisation,
  team, group and individual-test level.
- **FR-022**: The system MUST never reveal the content of a value marked confidential.
- **FR-023**: The system MUST provide at least one way to supply a confidential value that keeps it out
  of shell history and the process list.
- **FR-024**: The system MUST refuse a change to a shared value that another person has modified since it
  was read, and say so clearly.
- **FR-025**: Users MUST be able to retrieve files captured during authoring, such as screenshots.

**Authoring and export**

- **FR-026**: Users MUST be able to request a new test from a plain-language description, a starting
  address and a target browser or device.
- **FR-027**: Users MUST be able to watch authoring progress as it happens, seeing each attempted action
  and whether it succeeded.
- **FR-028**: Users MUST be able to reattach to authoring already in progress using a reference given
  when it started.
- **FR-029**: The system MUST bound how long it waits, and on expiry MUST report that the work may still
  be running.
- **FR-030**: Users MUST be able to discover which languages and frameworks a given test can be exported
  to, and export to any of them.
- **FR-031**: Exported source MUST be writable to a location the user chooses, and MUST NOT overwrite an
  existing file unless explicitly permitted.

**Cross-cutting**

- **FR-032**: The system MUST verify that the organisation is entitled to this capability before acting,
  and MUST distinguish "not included in your plan" from "could not be verified".
- **FR-033**: The system MUST work against every data centre where the capability is offered.
- **FR-034**: Every listing MUST offer both a human-readable and a machine-readable form, consistent with
  the tool's existing commands.
- **FR-035**: Every listing MUST indicate how many results exist beyond those shown.
- **FR-036**: When a requested item does not exist, the system MUST say so promptly rather than appearing
  to hang.
- **FR-037**: Removing a shared organisation-level asset — a test, group, schedule or shared value that
  colleagues may depend on — MUST require explicit confirmation before it is carried out.
- **FR-038**: The confirmation MUST state what is about to be removed and what else it affects, so the
  user can recognise a mistake before committing to it.
- **FR-039**: Users MUST be able to bypass confirmation explicitly, so removal remains usable in
  automation.
- **FR-040**: When confirmation cannot be sought — because the command is not being run interactively —
  and no explicit bypass was given, the system MUST refuse the removal and say why, rather than waiting
  for input that will never arrive or proceeding unconfirmed.
- **FR-041**: Confirmation behaviour MUST be consistent across all four shared asset types, so a user
  who learns it once can predict it everywhere.

### Key Entities

- **Authored Test**: A saved, reusable test produced by describing a journey in plain language. Has a
  name, labels, ownership and change history, may belong to a group, and carries default settings for
  where and how it runs.
- **Revision**: A point-in-time version of an authored test, capturing the original description, the
  sequence of actions derived from it, and the reasoning behind them.
- **Action**: A single step within a revision — navigating, clicking, entering text, scrolling, waiting,
  asserting and so on — recorded with the reasoning for it and, where captured, a screenshot.
- **Group**: A named collection of authored tests that can be run, scheduled and reported on together.
- **Schedule**: A recurring trigger that runs one or more groups on a repeating pattern in a stated
  timezone, optionally bounded by a start date, end date or maximum number of runs, and either active or
  suspended.
- **Shared Value**: A named value made available to tests, scoped to the whole organisation, a team, a
  group or a single test, and optionally confidential so its content is never disclosed.
- **Run**: One execution of an authored test, grouped under a build, producing one outcome per target.
- **Run Outcome**: The result of a run against a single browser or device, including whether it passed
  and a link to the full result.
- **Artifact**: A file captured during authoring or running, such as a screenshot.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A pipeline can be made to fail on an AI-authored test failure and pass otherwise, with no
  manual interpretation of output — verified by a run of each kind returning the correct status.
- **SC-002**: 100% of results returned for a named test belong to that test; a request for one test's
  history never includes another's.
- **SC-003**: A confidential value's content is never displayed by any command, and can be supplied by at
  least two routes that leave no trace in shell history.
- **SC-004**: An engineer can go from a plain-language description to a saved, runnable test without
  leaving the terminal.
- **SC-005**: An engineer can obtain reviewable source code for an authored test in their own language
  and framework in a single command.
- **SC-006**: Every command completes or reports a definite outcome; no command waits indefinitely, and
  every wait can be abandoned with clear instructions for resuming.
- **SC-007**: Asking for something that does not exist is answered as quickly as asking for something
  that does; a user never waits noticeably longer to be told "not found".
- **SC-008**: Machine-readable result reports contain one entry per executed test, so downstream report
  consumers display real results rather than empty containers.
- **SC-009**: Every capability offered by the underlying service is reachable from the command line, with
  no gaps that force a user back to the web interface.
- **SC-010**: An engineer already familiar with the tool's other commands can use these without new
  concepts — flag names, output choices and result formatting match what they already know.
- **SC-011**: Concurrent execution honours the configured limit, so a run cannot exhaust the
  organisation's capacity unintentionally.
- **SC-012**: No shared asset can be removed without either an explicit confirmation or an explicit
  bypass — verified across all four asset types, interactively and non-interactively.

## Assumptions

- The organisation holds a Sauce Labs plan that includes AI test authoring; access is verified before any
  action, and absence of it is a clear, actionable message rather than an error.
- Authoring in the web interface and the IDE plugins continues in parallel; this feature is an additional
  entry point, not a replacement, and must interoperate with tests created elsewhere.
- Existing credential handling and data-centre selection are reused unchanged; no new form of sign-in is
  introduced.
- Editing the recorded steps of an existing test is out of scope — the underlying service offers no such
  capability. Tests are re-authored or exported to code instead.
- Retrying a failed authored test is out of scope for this version; the tool's existing retry setting
  does not apply and its presence will be warned about rather than silently ignored.
- Configuration for this test type follows the same file, structure and conventions as the tool's
  existing test types, so an engineer who has configured one can configure this.
- Results appear in the same dashboards as every other test, so no new reporting surface is required.
- Command-line output conventions, table formatting and machine-readable output follow existing
  precedent in the tool rather than introducing a new style.
- Confirmation before removal is a deliberate departure from the tool's existing delete commands, which
  act immediately. It is justified by these assets being shared team state rather than
  individually-owned files, and is applied consistently across all four asset types so the exception is
  predictable rather than arbitrary.
