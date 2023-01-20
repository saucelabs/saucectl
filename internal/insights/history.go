package insights

// JobHistory represents job history data structure
type JobHistory struct {
	TestCases []TestCase `json:"test_cases"`
}

// TestCase represents test case data structure
type TestCase struct {
	Name     string  `json:"name"`
	FailRate float64 `json:"fail_rate"`
}
