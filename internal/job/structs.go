package job

// Details represent API response regarding job
type Details struct {
	BrowserShortVersion string `json:"browser_short_version"`
	VideoURL string `json:"video_url"`
	CreationTime int `json:"creation_time"`
	CustomData interface{} `json:"custom-data"`
	BrowserVersion string `json:"browser_version"`
	Owner string `json:"owner"`
	AutomationBackend string `json:"automation_backend"`
	ID string `json:"id"`
	CollectsAutomatorLog bool `json:"collects_automator_log"`
	RecordScreenshots bool `json:"record_screenshots"`
	RecordVideo bool `json:"record_video"`
	Build interface{} `json:"build"`
	Passed interface{} `json:"passed"`
	Public interface{} `json:"public"`
	AssignedTunnelID interface{} `json:"public"`
	Status string `json:"status"`
	LogURL string `json:"log_url"`
	StartTime int `json:"start_time"`
	Proxied bool `json:"proxied"`
	ModificationTime int `json:"modification_time"`
	Tags []string `json:"tags"`
	Name interface{} `json:"name"`
	CommandsNotSuccessful int `json:"commands_not_successful"`
	ConsolidatedStatus string `json:"consolidated_status"`
	SeleniumVersion interface{} `json:"selenium_version"`
	Manual bool `json:"manual"`
	EndTime int `json:"end_time"`
	Error interface{} `json:"error"`
	OS string `json:"os"`
	Breakpointed interface{} `json:"breakpointed"`
	Browser string `json:"browser"`
}
