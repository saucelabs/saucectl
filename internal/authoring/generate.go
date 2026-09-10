package authoring

import "fmt"

// GenerateOptions is the request to author a new test case: the AI agent
// drives a browser against the target, following the plain-language intent,
// and saves the result as a test case.
type GenerateOptions struct {
	// Name is the new test case's name, 1–255 characters.
	Name string `json:"name"`
	// TestSuiteID assigns the result to a suite. Dashed or dashless UUID.
	TestSuiteID string `json:"testSuiteId,omitempty"`
	// Tags has at most 20 entries of at most 60 characters each.
	Tags           []string            `json:"tags,omitempty"`
	RunSettings    GenerateRunSettings `json:"runSettings"`
	PromptSettings PromptSettings      `json:"promptSettings"`
	// TimeoutMillis is the service-side generation budget in milliseconds,
	// 60 000 to 3 600 000 (one minute to one hour). Zero omits it and lets the
	// service apply its default. This is distinct from the client-side wait
	// timeout, which bounds how long saucectl watches the task.
	TimeoutMillis int `json:"timeout,omitempty"`
}

// GenerateRunSettings is where the authoring session runs. Note the single
// Target, unlike the stored RunSettings' primary + run targets.
type GenerateRunSettings struct {
	Target Target `json:"target"`
	// TestURL is the starting address, at most 2048 characters.
	TestURL    string `json:"testUrl,omitempty"`
	TunnelName string `json:"scTunnelName,omitempty"`
}

// PromptSettings is what the agent is asked to do.
type PromptSettings struct {
	// Intent is the plain-language description, 1–20 000 characters.
	Intent string `json:"intent"`
	// MaxSteps caps the agent's actions, 1–200. Zero omits it.
	MaxSteps int `json:"maxSteps,omitempty"`
}

// GenerateTask identifies an accepted authoring task. TaskID is what
// GenerationStatus polls; SauceJobID is the browser session doing the work.
type GenerateTask struct {
	TaskID     string `json:"taskId"`
	SauceJobID string `json:"sauceJobId"`
}

// GenerationStatus is the lifecycle state of an authoring task.
type GenerationStatus string

// The generation statuses. QUEUED and IN_PROGRESS are transient; the service
// recommends polling every 2–3 seconds until COMPLETED or FAILED.
const (
	GenerationQueued     GenerationStatus = "QUEUED"
	GenerationInProgress GenerationStatus = "IN_PROGRESS"
	GenerationCompleted  GenerationStatus = "COMPLETED"
	GenerationFailed     GenerationStatus = "FAILED"
)

// GenerationState is the current state of an authoring task. Which fields are
// populated depends on Status: Steps and Reasoning while in progress and on
// failure, TestCaseID on completion, Error on failure.
type GenerationState struct {
	Status GenerationStatus `json:"status"`
	// Steps are the actions attempted so far. They are partial while the task
	// runs; the saved test case holds the final steps.
	Steps     []GenerationStep `json:"steps,omitempty"`
	Reasoning []Reasoning      `json:"reasoning,omitempty"`
	// TestCaseID is the saved test case, set only on completion.
	TestCaseID string `json:"testCaseId,omitempty"`
	// Error is set only on failure.
	Error *GenerationError `json:"error,omitempty"`
}

// Done reports whether the task has reached a terminal status.
func (s GenerationState) Done() bool {
	return s.Status == GenerationCompleted || s.Status == GenerationFailed
}

// GenerationStep is one action attempted during authoring. The wire name is
// "action" here, whereas a saved step calls the same thing "tool".
type GenerationStep struct {
	Action Tool        `json:"action"`
	Result *StepResult `json:"result,omitempty"`
}

// GenerationError is why an authoring task failed.
type GenerationError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// Error renders the code and detail.
func (e GenerationError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("generation failed: %s", e.Code)
	}
	return fmt.Sprintf("generation failed: %s: %s", e.Code, e.Detail)
}
