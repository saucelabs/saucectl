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
	"github.com/xtgo/uuid"

	"github.com/saucelabs/saucectl/internal/apitesting"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/msg"
	"github.com/saucelabs/saucectl/internal/region"
	"github.com/saucelabs/saucectl/internal/report"
	"github.com/saucelabs/saucectl/internal/tunnel"
)

var pollDefaultWait = time.Second * 180
var pollWaitTime = time.Second * 5

var unitFileName = "unit.yaml"
var inputFileName = "input.yaml"

// Runner represents an executor for api tests
type Runner struct {
	Project       Project
	Client        apitesting.Client
	Region        region.Region
	Reporters     []report.Reporter
	Async         bool
	TunnelService tunnel.Service
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
	if err := tunnel.ValidateTunnel(r.TunnelService, r.Project.Sauce.Tunnel.Name, r.Project.Sauce.Tunnel.Owner, tunnel.V2AlphaFilter, false); err != nil {
		return 1, err
	}

	passed := r.runSuites()
	if passed {
		exitCode = 0
	}
	return exitCode, nil
}

func hasUnitInputFiles(dir string) bool {
	st, err := os.Stat(path.Join(dir, unitFileName))
	if err != nil || st.IsDir() {
		return false
	}

	st, err = os.Stat(path.Join(dir, inputFileName))
	if err != nil || st.IsDir() {
		return false
	}
	return true
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
		if !hasUnitInputFiles(path) {
			return nil
		}

		relPath, _ := filepath.Rel(rootDir, path)
		if matchPath(relPath, testMatch) {
			tests = append(tests, relPath)
		}
		return nil
	}

	if err := filepath.WalkDir(rootDir, walker); err != nil {
		return []string{}, err
	}
	return tests, nil
}

func loadTest(unitPath string, inputPath string, suiteName string, testName string, tags []string, env map[string]string) (apitesting.TestRequest, error) {
	unitContent, err := os.ReadFile(unitPath)
	if err != nil {
		return apitesting.TestRequest{}, err
	}
	inputContent, err := os.ReadFile(inputPath)
	if err != nil {
		return apitesting.TestRequest{}, err
	}
	return apitesting.TestRequest{
		Name:   fmt.Sprintf("%s - %s", suiteName, testName),
		Tags:   append([]string{}, tags...),
		Input:  string(inputContent),
		Unit:   string(unitContent),
		Params: env,
	}, nil
}

func (r *Runner) loadTests(s Suite, tests []string) []apitesting.TestRequest {
	var testRequests []apitesting.TestRequest

	for _, test := range tests {
		req, err := loadTest(
			path.Join(r.Project.RootDir, test, unitFileName),
			path.Join(r.Project.RootDir, test, inputFileName),
			s.Name,
			test,
			s.Tags,
			s.Env,
		)
		if err != nil {
			log.Warn().
				Str("testName", test).
				Err(err).
				Msg("Unable to load test.")
		}
		testRequests = append(testRequests, req)
	}
	return testRequests
}

func (r *Runner) runLocalTests(s Suite, results chan []apitesting.TestResult) int {
	expected := 0
	taskID := uuid.NewRandom().String()

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
	tests := r.loadTests(s, matchingTests)

	for _, test := range tests {
		log.Info().
			Str("hookId", s.HookID).
			Str("testName", test.Name).
			Msg("Running test.")

		resp, err := r.Client.RunEphemeralAsync(context.Background(), s.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, taskID, test)
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

	if r.Async {
		r.buildLocalTestDetails(s.HookID, eventIDs, testNames, results)
	} else {
		r.startPollingAsyncResponse(s.HookID, eventIDs, results, maximumWaitTime)
	}
	return expected
}

func (r *Runner) runRemoteTests(s Suite, results chan []apitesting.TestResult) int {
	expected := 0
	maximumWaitTime := pollDefaultWait
	if s.Timeout != 0 {
		pollWaitTime = s.Timeout
	}

	if len(s.Tags) == 0 && len(s.Tests) == 0 {
		log.Info().Str("hookId", s.HookID).Msg("Running project.")

		resp, err := r.Client.RunAllAsync(context.Background(), s.HookID, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, apitesting.TestRequest{Params: s.Env})
		if err != nil {
			log.Error().Err(err).Msg("Failed to run project.")
		}

		if r.Async {
			r.fetchTestDetails(s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		return len(resp.EventIDs)
	}

	for _, t := range s.Tests {
		test := t
		log.Info().Str("test", test).Str("hookId", s.HookID).Msg("Running test.")

		resp, err := r.Client.RunTestAsync(context.Background(), s.HookID, test, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, apitesting.TestRequest{Params: s.Env})

		if err != nil {
			log.Error().Err(err).Msg("Failed to run test.")
		}
		if r.Async {
			r.fetchTestDetails(s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		expected += len(resp.EventIDs)
	}

	for _, t := range s.Tags {
		tag := t
		log.Info().Str("tag", tag).Str("hookId", s.HookID).Msg("Running tag.")

		resp, err := r.Client.RunTagAsync(context.Background(), s.HookID, tag, r.Project.Sauce.Metadata.Build, r.Project.Sauce.Tunnel, apitesting.TestRequest{Params: s.Env})
		if err != nil {
			log.Error().Err(err).Msg("Failed to run tag.")
		}
		if r.Async {
			r.fetchTestDetails(s.HookID, resp.EventIDs, resp.TestIDs, results)
		} else {
			r.startPollingAsyncResponse(s.HookID, resp.EventIDs, results, maximumWaitTime)
		}
		expected += len(resp.EventIDs)
	}
	return expected
}

func (r *Runner) runSuites() bool {
	results := make(chan []apitesting.TestResult)
	expected := 0

	for _, s := range r.Project.Suites {
		suite := s
		log.Info().
			Str("hookId", suite.HookID).
			Str("suite", suite.Name).
			Bool("parallel", true).
			Msg("Starting suite")

		if s.UseRemoteTests {
			expected += r.runRemoteTests(s, results)
		} else {
			expected += r.runLocalTests(s, results)
		}

	}
	return r.collectResults(expected, results)
}

func (r *Runner) buildLocalTestDetails(hookID string, eventIDs []string, testNames []string, results chan []apitesting.TestResult) {
	project, _ := r.Client.GetProject(context.Background(), hookID)
	for _, eventID := range eventIDs {
		reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)
		log.Info().
			Str("project", project.Name).
			Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
			Str("report", reportURL).
			Msg("Async test started.")
	}

	for _, testName := range testNames {
		go func(p apitesting.Project, testID string) {
			results <- []apitesting.TestResult{{
				Test:    apitesting.Test{Name: testID},
				Project: p,
				Async:   true,
			}}
		}(project, testName)
	}
}

func (r *Runner) fetchTestDetails(hookID string, eventIDs []string, testIDs []string, results chan []apitesting.TestResult) {
	project, _ := r.Client.GetProject(context.Background(), hookID)
	for _, eventID := range eventIDs {
		reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)
		log.Info().
			Str("project", project.Name).
			Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, eventID)).
			Str("report", reportURL).
			Msg("Async test started.")
	}

	for _, testID := range testIDs {
		go func(p apitesting.Project, testID string) {
			test, _ := r.Client.GetTest(context.Background(), hookID, testID)
			results <- []apitesting.TestResult{{
				Test:    test,
				Project: p,
				Async:   true,
			}}
		}(project, testID)
	}
}

func (r *Runner) startPollingAsyncResponse(hookID string, eventIDs []string, results chan []apitesting.TestResult, pollMaximumWait time.Duration) {
	project, _ := r.Client.GetProject(context.Background(), hookID)

	for _, eventID := range eventIDs {
		go func(lEventID string) {
			timeout := (time.Now()).Add(pollMaximumWait)

			for {
				result, err := r.Client.GetEventResult(context.Background(), hookID, lEventID)

				if err == nil {
					log.Info().
						Str("hookId", hookID).
						Str("testName", result.Test.Name).
						Msg("Finished test.")
					results <- []apitesting.TestResult{result}
					break
				}
				if err.Error() != "event not found" {
					results <- []apitesting.TestResult{{
						EventID:       lEventID,
						FailuresCount: 1,
					}}
					break
				}
				if timeout.Before(time.Now()) {
					reportURL := fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, lEventID)
					log.Warn().
						Str("project", project.Name).
						Str("report", fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), project.ID, lEventID)).
						Str("report", reportURL).
						Msg("Test did not finish before timeout.")
					results <- []apitesting.TestResult{{
						Project:  project,
						EventID:  lEventID,
						Async:    true,
						TimedOut: true,
					}}
					break
				}
				time.Sleep(pollWaitTime)
			}
		}(eventID)
	}
}

func (r *Runner) collectResults(expected int, results chan []apitesting.TestResult) bool {
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
		res := <-results

		inProgress--

		for _, testResult := range res {
			var reportURL string
			testName := buildTestName(testResult.Project, testResult.Test)

			if !testResult.Async {
				reportURL = fmt.Sprintf("%s/api-testing/project/%s/event/%s", r.Region.AppBaseURL(), testResult.Project.ID, testResult.EventID)

				log.Info().
					Int("failures", testResult.FailuresCount).
					Str("project", testResult.Project.Name).
					Str("report", reportURL).
					Str("test", testResult.Test.Name).
					Msg("Finished test.")
			}

			status := job.StatePassed
			if testResult.FailuresCount > 0 || testResult.TimedOut {
				status = job.StateFailed
				passed = false
			} else if testResult.Async {
				status = job.StateInProgress
			}

			for _, rep := range r.Reporters {
				rep.Add(report.TestResult{
					Name:      testName,
					URL:       reportURL,
					Status:    status,
					Duration:  time.Second * time.Duration(testResult.ExecutionTimeSeconds),
					StartTime: (time.Now()).Add(-time.Second * time.Duration(testResult.ExecutionTimeSeconds)),
					Attempts:  1,
				})
			}
		}
	}
	close(done)

	for _, rep := range r.Reporters {
		rep.Render()
	}

	return passed
}

func buildTestName(project apitesting.Project, test apitesting.Test) string {
	if test.Name != "" {
		return fmt.Sprintf("%s - %s", project.Name, test.Name)
	}
	return project.Name
}

// ResolveHookIDs resolve, for each suite, the matching hookID.
func (r *Runner) ResolveHookIDs() error {
	hookIDMappings := map[string]apitesting.Hook{}
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
			log.Error().Str("suiteName", s.Name).Msgf(msg.ProjectNotFound, s.ProjectName)
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
				hasErrors = true
				continue
			}

			hook = hooks[0]
			hookIDMappings[project.ID] = hooks[0]
		}

		log.Info().Msgf(msg.HookUsedForSuite, hook.Identifier, s.Name)
		r.Project.Suites[idx].HookID = hook.Identifier
	}

	if hasErrors {
		return errors.New(msg.FailedToPrepareSuites)
	}
	return nil
}

func findMatchingProject(name string, projects []apitesting.Project) apitesting.Project {
	for _, p := range projects {
		if p.Name == name {
			return p
		}
	}
	return apitesting.Project{}
}
