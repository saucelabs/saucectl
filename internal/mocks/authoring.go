package mocks

import (
	"context"
	"io"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// AuthoringService is a hand-written fake for every authoring service
// interface. Each method delegates to its Fn field; an unset field panics,
// which makes an unexpected call in a test loud rather than silently
// successful.
type AuthoringService struct {
	ListTestCasesFn    func(ctx context.Context, opts authoring.ListTestCasesOptions) (authoring.List[authoring.TestCase], error)
	GetTestCaseFn      func(ctx context.Context, id string) (authoring.TestCase, error)
	DeleteTestCaseFn   func(ctx context.Context, id string) error
	RenameTestCaseFn   func(ctx context.Context, id, name string) (authoring.TestCase, error)
	RunTestCaseFn      func(ctx context.Context, id, revisionID string, opts authoring.RunOptions) (authoring.Run, error)
	ListRunsFn         func(ctx context.Context, testCaseID string, opts authoring.ListRunsOptions) (authoring.List[authoring.Run], error)
	GetRunFn           func(ctx context.Context, testCaseID, runID string) (authoring.Run, error)
	ListTagsFn         func(ctx context.Context) ([]string, error)
	GenerateFn         func(ctx context.Context, opts authoring.GenerateOptions) (authoring.GenerateTask, error)
	GenerationStatusFn func(ctx context.Context, taskID string) (authoring.GenerationState, error)
	CodeFn             func(ctx context.Context, id, target string) (string, error)
	CodeTargetsFn      func(ctx context.Context, id string) ([]string, error)

	ListTestSuitesFn  func(ctx context.Context, opts authoring.ListTestSuitesOptions) (authoring.List[authoring.TestSuite], error)
	GetTestSuiteFn    func(ctx context.Context, id string) (authoring.TestSuite, error)
	CreateTestSuiteFn func(ctx context.Context, opts authoring.CreateTestSuiteOptions) (authoring.TestSuite, error)
	UpdateTestSuiteFn func(ctx context.Context, id string, opts authoring.UpdateTestSuiteOptions) (authoring.TestSuite, error)
	DeleteTestSuiteFn func(ctx context.Context, id string, deleteTestCases bool) error
	RunTestSuiteFn    func(ctx context.Context, id, buildName string) (authoring.SuiteRun, error)

	ListSchedulesFn  func(ctx context.Context, opts authoring.ListSchedulesOptions) (authoring.List[authoring.TestSchedule], error)
	GetScheduleFn    func(ctx context.Context, id string) (authoring.TestSchedule, error)
	CreateScheduleFn func(ctx context.Context, opts authoring.CreateScheduleOptions) (authoring.TestSchedule, error)
	UpdateScheduleFn func(ctx context.Context, id string, opts authoring.UpdateScheduleOptions) (authoring.TestSchedule, error)
	DeleteScheduleFn func(ctx context.Context, id string) error

	ListVariablesFn  func(ctx context.Context, opts authoring.ListVariablesOptions) (authoring.List[authoring.Variable], error)
	GetVariableFn    func(ctx context.Context, id string) (authoring.Variable, error)
	CreateVariableFn func(ctx context.Context, opts authoring.CreateVariableOptions) (authoring.Variable, error)
	UpdateVariableFn func(ctx context.Context, id string, opts authoring.UpdateVariableOptions) (authoring.Variable, error)
	DeleteVariableFn func(ctx context.Context, id, expectedLastUpdate string) error

	DownloadArtifactFn func(ctx context.Context, id string) (io.ReadCloser, error)

	IsAIAuthoringEnabledFn func(ctx context.Context, orgID string) (bool, error)
}

// ListTestCases delegates to ListTestCasesFn.
func (s *AuthoringService) ListTestCases(ctx context.Context, opts authoring.ListTestCasesOptions) (authoring.List[authoring.TestCase], error) {
	return s.ListTestCasesFn(ctx, opts)
}

// GetTestCase delegates to GetTestCaseFn.
func (s *AuthoringService) GetTestCase(ctx context.Context, id string) (authoring.TestCase, error) {
	return s.GetTestCaseFn(ctx, id)
}

// DeleteTestCase delegates to DeleteTestCaseFn.
func (s *AuthoringService) DeleteTestCase(ctx context.Context, id string) error {
	return s.DeleteTestCaseFn(ctx, id)
}

// RenameTestCase delegates to RenameTestCaseFn.
func (s *AuthoringService) RenameTestCase(ctx context.Context, id, name string) (authoring.TestCase, error) {
	return s.RenameTestCaseFn(ctx, id, name)
}

// RunTestCase delegates to RunTestCaseFn.
func (s *AuthoringService) RunTestCase(ctx context.Context, id, revisionID string, opts authoring.RunOptions) (authoring.Run, error) {
	return s.RunTestCaseFn(ctx, id, revisionID, opts)
}

// ListRuns delegates to ListRunsFn.
func (s *AuthoringService) ListRuns(ctx context.Context, testCaseID string, opts authoring.ListRunsOptions) (authoring.List[authoring.Run], error) {
	return s.ListRunsFn(ctx, testCaseID, opts)
}

// GetRun delegates to GetRunFn.
func (s *AuthoringService) GetRun(ctx context.Context, testCaseID, runID string) (authoring.Run, error) {
	return s.GetRunFn(ctx, testCaseID, runID)
}

// ListTags delegates to ListTagsFn.
func (s *AuthoringService) ListTags(ctx context.Context) ([]string, error) {
	return s.ListTagsFn(ctx)
}

// Generate delegates to GenerateFn.
func (s *AuthoringService) Generate(ctx context.Context, opts authoring.GenerateOptions) (authoring.GenerateTask, error) {
	return s.GenerateFn(ctx, opts)
}

// GenerationStatus delegates to GenerationStatusFn.
func (s *AuthoringService) GenerationStatus(ctx context.Context, taskID string) (authoring.GenerationState, error) {
	return s.GenerationStatusFn(ctx, taskID)
}

// Code delegates to CodeFn.
func (s *AuthoringService) Code(ctx context.Context, id, target string) (string, error) {
	return s.CodeFn(ctx, id, target)
}

// CodeTargets delegates to CodeTargetsFn.
func (s *AuthoringService) CodeTargets(ctx context.Context, id string) ([]string, error) {
	return s.CodeTargetsFn(ctx, id)
}

// ListTestSuites delegates to ListTestSuitesFn.
func (s *AuthoringService) ListTestSuites(ctx context.Context, opts authoring.ListTestSuitesOptions) (authoring.List[authoring.TestSuite], error) {
	return s.ListTestSuitesFn(ctx, opts)
}

// GetTestSuite delegates to GetTestSuiteFn.
func (s *AuthoringService) GetTestSuite(ctx context.Context, id string) (authoring.TestSuite, error) {
	return s.GetTestSuiteFn(ctx, id)
}

// CreateTestSuite delegates to CreateTestSuiteFn.
func (s *AuthoringService) CreateTestSuite(ctx context.Context, opts authoring.CreateTestSuiteOptions) (authoring.TestSuite, error) {
	return s.CreateTestSuiteFn(ctx, opts)
}

// UpdateTestSuite delegates to UpdateTestSuiteFn.
func (s *AuthoringService) UpdateTestSuite(ctx context.Context, id string, opts authoring.UpdateTestSuiteOptions) (authoring.TestSuite, error) {
	return s.UpdateTestSuiteFn(ctx, id, opts)
}

// DeleteTestSuite delegates to DeleteTestSuiteFn.
func (s *AuthoringService) DeleteTestSuite(ctx context.Context, id string, deleteTestCases bool) error {
	return s.DeleteTestSuiteFn(ctx, id, deleteTestCases)
}

// RunTestSuite delegates to RunTestSuiteFn.
func (s *AuthoringService) RunTestSuite(ctx context.Context, id, buildName string) (authoring.SuiteRun, error) {
	return s.RunTestSuiteFn(ctx, id, buildName)
}

// ListSchedules delegates to ListSchedulesFn.
func (s *AuthoringService) ListSchedules(ctx context.Context, opts authoring.ListSchedulesOptions) (authoring.List[authoring.TestSchedule], error) {
	return s.ListSchedulesFn(ctx, opts)
}

// GetSchedule delegates to GetScheduleFn.
func (s *AuthoringService) GetSchedule(ctx context.Context, id string) (authoring.TestSchedule, error) {
	return s.GetScheduleFn(ctx, id)
}

// CreateSchedule delegates to CreateScheduleFn.
func (s *AuthoringService) CreateSchedule(ctx context.Context, opts authoring.CreateScheduleOptions) (authoring.TestSchedule, error) {
	return s.CreateScheduleFn(ctx, opts)
}

// UpdateSchedule delegates to UpdateScheduleFn.
func (s *AuthoringService) UpdateSchedule(ctx context.Context, id string, opts authoring.UpdateScheduleOptions) (authoring.TestSchedule, error) {
	return s.UpdateScheduleFn(ctx, id, opts)
}

// DeleteSchedule delegates to DeleteScheduleFn.
func (s *AuthoringService) DeleteSchedule(ctx context.Context, id string) error {
	return s.DeleteScheduleFn(ctx, id)
}

// ListVariables delegates to ListVariablesFn.
func (s *AuthoringService) ListVariables(ctx context.Context, opts authoring.ListVariablesOptions) (authoring.List[authoring.Variable], error) {
	return s.ListVariablesFn(ctx, opts)
}

// GetVariable delegates to GetVariableFn.
func (s *AuthoringService) GetVariable(ctx context.Context, id string) (authoring.Variable, error) {
	return s.GetVariableFn(ctx, id)
}

// CreateVariable delegates to CreateVariableFn.
func (s *AuthoringService) CreateVariable(ctx context.Context, opts authoring.CreateVariableOptions) (authoring.Variable, error) {
	return s.CreateVariableFn(ctx, opts)
}

// UpdateVariable delegates to UpdateVariableFn.
func (s *AuthoringService) UpdateVariable(ctx context.Context, id string, opts authoring.UpdateVariableOptions) (authoring.Variable, error) {
	return s.UpdateVariableFn(ctx, id, opts)
}

// DeleteVariable delegates to DeleteVariableFn.
func (s *AuthoringService) DeleteVariable(ctx context.Context, id, expectedLastUpdate string) error {
	return s.DeleteVariableFn(ctx, id, expectedLastUpdate)
}

// DownloadArtifact delegates to DownloadArtifactFn.
func (s *AuthoringService) DownloadArtifact(ctx context.Context, id string) (io.ReadCloser, error) {
	return s.DownloadArtifactFn(ctx, id)
}

// IsAIAuthoringEnabled delegates to IsAIAuthoringEnabledFn.
func (s *AuthoringService) IsAIAuthoringEnabled(ctx context.Context, orgID string) (bool, error) {
	return s.IsAIAuthoringEnabledFn(ctx, orgID)
}
