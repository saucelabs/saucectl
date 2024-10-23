package apitest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

var pollDefaultWait = time.Second * 180
var pollWaitTime = time.Second * 5

var unitFileNames = []string{"unit.yaml", "unit.yml"}
var inputFileNames = []string{"input.yaml", "input.yml"}

type APITester interface {
	GetProject(ctx context.Context, hookID string) (ProjectMeta, error)
	GetEventResult(ctx context.Context, hookID string, eventID string) (TestResult, error)
	GetTest(ctx context.Context, hookID string, testID string) (Test, error)
	GetProjects(ctx context.Context) ([]ProjectMeta, error)
	GetHooks(ctx context.Context, projectID string) ([]Hook, error)
	RunAllAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunEphemeralAsync(ctx context.Context, hookID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunTestAsync(ctx context.Context, hookID string, testID string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
	RunTagAsync(ctx context.Context, hookID string, testTag string, buildID string, tunnel config.Tunnel, test TestRequest) (AsyncResponse, error)
}

// TestResult describes the result from running an api test.
type TestResult struct {
	EventID              string      `json:"_id,omitempty"`
	FailuresCount        int         `json:"failuresCount,omitempty"`
	Project              ProjectMeta `json:"project,omitempty"`
	Test                 Test        `json:"test,omitempty"`
	ExecutionTimeSeconds int         `json:"executionTimeSeconds,omitempty"`
	Async                bool        `json:"-"`
	TimedOut             bool        `json:"-"`
	Error                error       `json:"-"`
}

// ProjectMeta describes the metadata for an api testing project.
type ProjectMeta struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Hook describes the metadata for a hook.
type Hook struct {
	Identifier string `json:"identifier,omitempty"`
	Name       string `json:"name,omitempty"`
}

// Test describes a single test.
type Test struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// TestRequest represent a test to be executed
type TestRequest struct {
	Name   string            `json:"name"`
	Tags   []string          `json:"tags"`
	Input  string            `json:"input"`
	Unit   string            `json:"unit"`
	Params map[string]string `json:"params"`
}

// AsyncResponse describes the json response from the async api endpoints.
type AsyncResponse struct {
	ContextIDs []string `json:"contextIds,omitempty"`
	EventIDs   []string `json:"eventIds,omitempty"`
	TaskID     string   `json:"taskId,omitempty"`
	TestIDs    []string `json:"testIds,omitempty"`
}

// Runner represents an executor for api tests
type Runner struct {
	Project       Project
	Client        APITester
	Region        region.Region
	Reporters     []report.Reporter
	Async         bool
	TunnelService tunnel.Service
}

// Vault represents a project's stored variables and snippets
type Vault struct {
	Variables []VaultVariable   `json:"variables"`
	Snippets  map[string]string `json:"snippets"`
}

// VaultVariable represents a variable stored in a project vault
type VaultVariable struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	Type  string `json:"type,omitempty"`
}

// VaultFile represents a file stored in a project vault
type VaultFile struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	CompanyID string `json:"companyId"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Size      int    `json:"size"`
	Source    string `json:"source"`
	IsOpenAPI bool   `json:"isOpenAPI"`
}

// FilterSuites filters out suites in the project that don't match the given suite name.
func FilterSuites(p *Project, suiteName string) error {
	for _, s := range p.Suites {
		if s.Name == suiteName {
			p.Suites = []Suite{s}
			return nil
		}
	}
	return fmt.Errorf(msg.SuiteNameNotFound, suiteName)
}

// RunProject runs the tests defined in apitest.Project
func (r *Runner) RunProject() (int, error) {
	exitCode := 1
	if err := tunnel.Validate(
		r.TunnelService,
		r.Project.Sauce.Tunnel.Name,
		r.Project.Sauce.Tunnel.Owner,
		tunnel.V2AlphaFilter,
		false,
		r.Project.Sauce.Tunnel.Timeout,
	); err != nil {
		return 1, err
	}

	if r.Project.DryRun {
		printDryRunSuiteNames(r.getSuiteNames())
		return 0, nil
	}

	passed := r.runSuites()
	if passed {
		exitCode = 0
	}
	return exitCode, nil
}

func (r *Runner) getSuiteNames() []string {
	var names []string
	for _, s := range r.Project.Suites {
		names = append(names, s.Name)
	}
	return names
}

func printDryRunSuiteNames(suites []string) {
	fmt.Println("\nThe following test suites would have run:")
	for _, s := range suites {
		fmt.Printf("  - %s\n", s)
	}
	fmt.Println()
}

func hasUnitInputFiles(dir string) bool {
	_, err := findUnitFile(dir)
	hasUnitFile := err == nil

	_, err = findInputFile(dir)
	hasInputFile := err == nil

	return hasUnitFile && hasInputFile
}

func findInputFile(dir string) (string, error) {
	for _, n := range inputFileNames {
		p := path.Join(dir, n)
		st, err := os.Stat(p)

		if err == nil && !st.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("Failed to find any input file (%v) in dir (%s)", inputFileNames, dir)
}

func findUnitFile(dir string) (string, error) {
	for _, n := range unitFileNames {
		p := path.Join(dir, n)
		st, err := os.Stat(p)

		if err == nil && !st.IsDir() {
			return p, nil
		}
	}

	return "", fmt.Errorf("Failed to find any unit file (%v) in dir (%s)", unitFileNames, dir)
}

func matchPath(dir string, pathMatch []string) bool {
	if len(pathMatch) == 0 {
		return true
	}
	for _, v := range pathMatch {
		re, err := regexp.Compile(v)
		if err != nil {
			continue
		}
		if re.MatchString(dir) {
			return true
		}
	}
	return false
}

func findTests(rootDir string, testMatch []string) ([]string, error) {
	var tests []string

	walker := func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(rootDir, path)
		if matchPath(relPath, testMatch) {
			if !hasUnitInputFiles(path) {
				log.Warn().Msgf("Skipping matching path (%s): unit or input files missing.", path)
				return nil
			}
			tests = append(tests, relPath)
		}
		return nil
	}

	if err := filepath.WalkDir(rootDir, walker); err != nil {
		return []string{}, err
	}
	return tests, nil
}

func newTestRequest(testDir string, suiteName string, testName string, tags []string, env map[string]string) (TestRequest, error) {
	unitFile, err := findUnitFile(testDir)
	if err != nil {
		return TestRequest{}, err
	}
	unitContent, err := os.ReadFile(unitFile)
	if err != nil {
		return TestRequest{}, err
	}

	inputFile, err := findInputFile(testDir)
	if err != nil {
		return TestRequest{}, err
	}
	inputContent, err := os.ReadFile(inputFile)
	if err != nil {
		return TestRequest{}, err
	}

	return TestRequest{
		Name:   fmt.Sprintf("%s - %s", suiteName, testName),
		Tags:   append([]string{}, tags...),
		Input:  string(inputContent),
		Unit:   string(unitContent),
		Params: env,
	}, nil
}

func (r *Runner) newTestRequests(s Suite, tests []string) []TestRequest {
	var testRequests []TestRequest

	for _, test := range tests {
		req, err := newTestRequest(
			path.Join(r.Project.RootDir, test),
			s.Name,
			test,
			s.Tags,
			s.Env,
		)
		if err != nil {
			log.Warn().
				Str("testName", test).
				Err(err).
				Msg("Unable to open test.")
		}
		testRequests = append(testRequests, req)
	}
	return testRequests
}

func (r *Runner) runLocalTests(s Suite, results chan TestResult) int {
	expected := 0

	maximumWaitTime := pollDefaultWait
	if s.Timeout != 0 {
		pollWaitTime = s.Timeout
	}

	var eventIDs []string
	var testNames []string

	matchingTests, err := findTests(r.Project.RootDir, s.TestMatch)
	if err != nil {
		log.Error().Err(err).Str("rootDir", r.Project.RootDir).Msg("Unable to walk rootDir")
		return 0
	}
	tests := r.newTestRequests(s, matchingTests)
	if len(tests) == 0 {
		log.Warn().Msgf("Could not open local tests matching patterns (%v). See https://github.com/saucelabs/saucectl-apix-example/blob/main/docs/README.md for more details.", s.TestMatch)
		return 0
	}

	for _, test := range tests {
		log.Info().
			Str("projectName", s.ProjectName).
			Str("testName", test.Name).
			Msg("Running test.")

		resp, err := r.Client.RunEphemeralAsync(context.Background(), s.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, test)
		if err != nil {
			log.Error().
				Err(err).
				Str("testName", test.Name).
				Msg("Failed to run test.")
			continue
		}
		testNames = append(testNames, test.Name)
		eventIDs = append(eventIDs, resp.EventIDs...)
		expected++
	}

	projectMeta := ProjectMeta{
		ID:   s.ProjectID,
		Name: s.ProjectName,
	}

	if r.Async {
		r.buildLocalTestDetails(projectMeta, eventIDs, testNames, results)
	} else {
		r.startPollingAsyncResponse(projectMeta, s.HookID, eventIDs, results, maximumWaitTime)
	}
	return expected
}

func (r *Runner) runRemoteTests(s Suite, results chan TestResult) int {
	expected := 0
	maximumWaitTime := pollDefaultWait
	if s.Timeout != 0 {
		pollWaitTime = s.Timeout
	}

	projectMeta := ProjectMeta{
		ID:   s.ProjectID,
		Name: s.ProjectName,
	}

	if len(s.Tags) == 0 && len(s.Tests) == 0 {
		log.Info().Str("projectName", s.ProjectName).Msg("Running project.")

		resp, err := r.Client.RunAllAsync(context.Background(), s.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, TestRequest{Params: s.Env})
		if err != nil {
			log.Error().Err(err).Msg("Failed to run project.")
		}

		if r.Async {
			r.fetchTestDetails(projectMeta, s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(projectMeta, s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		return len(resp.EventIDs)
	}

	for _, t := range s.Tests {
		test := t
		log.Info().Str("test", test).Str("projectName", s.ProjectName).Msg("Running test.")

		resp, err := r.Client.RunTestAsync(context.Background(), s.HookID, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, TestRequest{Params: s.Env})

		if err != nil {
			log.Error().Err(err).Msg("Failed to run test.")
		}

		if r.Async {
			r.fetchTestDetails(projectMeta, s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(projectMeta, s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		expected += len(resp.EventIDs)
	}

	for _, t := range s.Tags {
		tag := t
		log.Info().Str("tag", tag).Str("projectName", s.ProjectName).Msg("Running tag.")

		resp, err := r.Client.RunTagAsync(context.Background(), s.HookID, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, TestRequest{Params: s.Env})
		if err != nil {
			log.Error().Err(err).Msg("Failed to run tag.")
		}
		if r.Async {
			r.fetchTestDetails(projectMeta, s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(projectMeta, s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		expected += len(resp.EventIDs)
	}
	return expected
}

func (r *Runner) runSuites() bool {
	results := make(chan TestResult)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s
		log.Info().
			Str("projectName", suite.ProjectName).
			Str("suite", suite.Name).
			Bool("parallel", true).
			Str("tunnel", r.Project.Sauce.Tunnel.Name).
			Msg("Starting suite")

		if s.UseRemoteTests {
			expected += r.runRemoteTests(s, results)
		} else {
			expected += r.runLocalTests(s, results)
		}

	}
	return r.collectResults(expected, results)
}

func (r *Runner) buildLocalTestDetails(project ProjectMeta, eventIDs []string, testNames []string, results chan TestResult) {
	for _, eventID := range eventIDs {
		log.Info().
			Str("project", project.Name).
			Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
			Msg("Async test started.")
	}

	for _, testName := range testNames {
		go func(p ProjectMeta, testID string) {
			results <- TestResult{
				Test:    Test{Name: testID},
				Project: p,
				Async:   true,
			}
		}(project, testName)
	}
}

func (r *Runner) fetchTestDetails(project ProjectMeta, hookID string, eventIDs []string, testIDs []string, results chan TestResult) {
	for _, eventID := range eventIDs {
		log.Info().
			Str("project", project.Name).
			Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
			Msg("Async test started.")
	}

	for _, testID := range testIDs {
		go func(p ProjectMeta, testID string) {
			test, _ := r.Client.GetTest(context.Background(), hookID, testID)
			results <- TestResult{
				Test:    test,
				Project: p,
				Async:   true,
			}
		}(project, testID)
	}
}

func (r *Runner) startPollingAsyncResponse(project ProjectMeta, hookID string, eventIDs []string, results chan TestResult, pollMaximumWait time.Duration) {
	for _, eventID := range eventIDs {
		go func(lEventID string) {
			deadline := time.NewTimer(pollMaximumWait)
			ticker := time.NewTicker(pollWaitTime)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					result, err := r.Client.GetEventResult(context.Background(), hookID, lEventID)

					// Events are not available when the test is still running.
					if err == ErrEventNotFound {
						continue
					}

					if err != nil {
						results <- TestResult{
							EventID:       lEventID,
							Project:       project,
							FailuresCount: 1,
							Error:         err,
						}
						return
					}

					results <- result
					return
				case <-deadline.C:
					results <- TestResult{
						Project:  project,
						EventID:  lEventID,
						TimedOut: true,
					}
					return
				}
			}
		}(eventID)
	}
}

func (r *Runner) collectResults(expected int, results chan TestResult) bool {
	inProgress := expected
	passed := true

	done := make(chan interface{})
	go func(r *Runner) {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				log.Info().Msgf("Tests in progress: %d", inProgress)
			}
		}
	}(r)

	for i := 0; i < expected; i++ {
		testResult := <-results

		inProgress--

		var reportURL string
		testName := buildTestName(testResult.Project, testResult.Test)

		if !testResult.Async {
			reportURL = fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)

			logEvent := log.Info()
			logMsg := "Test finished."
			if testResult.FailuresCount > 0 || testResult.TimedOut {
				logEvent = log.Error()
				logMsg = "Test finished with errors."
			}
			logEvent.
				Err(testResult.Error).
				Int("failures", testResult.FailuresCount).
				Str("project", testResult.Project.Name).
				Str("report", reportURL).
				Str("test", testResult.Test.Name)

			logEvent.Msg(logMsg)
		}

		status := job.StatePassed
		if testResult.FailuresCount > 0 {
			status = job.StateFailed
		} else if testResult.Async {
			status = job.StateInProgress
		}

		if status == job.StateFailed || testResult.TimedOut {
			passed = false
		}

		duration := time.Duration(testResult.ExecutionTimeSeconds) * time.Second
		startTime := time.Now().Add(-duration)
		endTime := time.Now()

		for _, rep := range r.Reporters {
			rep.Add(report.TestResult{
				Name:      testName,
				URL:       reportURL,
				Status:    status,
				Duration:  duration,
				StartTime: startTime,
				EndTime:   endTime,
				Attempts: []report.Attempt{{
					Duration:  duration,
					StartTime: startTime,
					EndTime:   endTime,
					Status:    status,
				}},
				TimedOut: testResult.TimedOut,
			})
		}
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}

	return passed
}

func buildTestName(project ProjectMeta, test Test) string {
	if test.Name != "" {
		return fmt.Sprintf("%s - %s", project.Name, test.Name)
	}
	return project.Name
}

// ResolveHookIDs resolve, for each suite, the matching hookID.
func (r *Runner) ResolveHookIDs() error {
	hookIDMappings := map[string]Hook{}
	hasErrors := false

	projects, err := r.Client.GetProjects(context.Background())
	if err != nil {
		log.Error().Err(err).Msg(msg.ProjectListFailure)
		return err
	}

	for idx, s := range r.Project.Suites {
		if s.HookID != "" {
			continue
		}

		project := findMatchingProject(s.ProjectName, projects)
		if project.ID == "" {
			log.Error().Str("suiteName", s.Name).Msgf(msg.ProjectNotFound, s.ProjectName, r.Project.Sauce.Region)
			hasErrors = true
			continue
		}

		hook := hookIDMappings[project.ID]

		if hook.Identifier == "" {
			hooks, err := r.Client.GetHooks(context.Background(), project.ID)

			if err != nil {
				log.Err(err).Str("suiteName", s.Name).Msg(msg.HookQueryFailure)
				hasErrors = true
				continue
			}
			if len(hooks) == 0 {
				log.Error().Str("suiteName", s.Name).Msgf(msg.NoHookForProject, project.Name)
				creationLink := fmt.Sprintf("%s/api-testing/project/%s/hook", r.Region.AppBaseURL(), project.ID)
				fmt.Printf(msg.WebhookCreationLink, creationLink, project.Name)
				fmt.Printf("\n\n")
				hasErrors = true
				continue
			}

			hook = hooks[0]
			hookIDMappings[project.ID] = hooks[0]
		}

		r.Project.Suites[idx].HookID = hook.Identifier
		r.Project.Suites[idx].ProjectID = project.ID
	}

	if hasErrors {
		return errors.New(msg.FailedToPrepareSuites)
	}
	return nil
}

func findMatchingProject(name string, projects []ProjectMeta) ProjectMeta {
	for _, p := range projects {
		if p.Name == name {
			return p
		}
	}
	return ProjectMeta{}
}
