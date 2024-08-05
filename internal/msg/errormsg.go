package msg

// cmd setting
const (
	// InvalidUsername indicates invalid username
	InvalidUsername = "invalid username"
	// EmptyUsername asks user to type a username
	EmptyUsername = "you need to type a username"
	// InvalidAccessKey indicates invalid key
	InvalidAccessKey = "invalid access key"
	// EmptyAccessKey asks user to type an access key
	EmptyAccessKey = "you need to type an access key"
	// EmptyCredentials indicates no credentials
	EmptyCredentials = "no credentials available"
	// InvalidSelectedFramework indicates invalid framework
	InvalidSelectedFramework = "invalid framework selected"
	// InvalidCredentials indicates invalid credentials
	InvalidCredentials = "invalid credentials provided"
	// UnableToCheckCredentials
	UnableToCheckCredentials = "unable to check credentials"
	// MissingFrameworkVersion indicates empty framework version
	MissingFrameworkVersion = "no %s version specified"
	// MissingCypressConfig indicates no cypress config file
	MissingCypressConfig = "no cypress config file specified"
	// MissingPlatformName indicates no platform name
	MissingPlatformName = "no platform name specified"
	// MissingBrowserName indicates no browser name
	MissingBrowserName = "no browser name specified"
	// MissingDockerImage indicates no app
	MissingDockerImage = "no image provided"
	// MissingApp indicates no app
	MissingApp = "no app provided"
	// MissingTestApp indicates no testApp
	MissingTestApp = "no testApp provided"
	// MissingDeviceOrEmulator indicates no device or emulator
	MissingDeviceOrEmulator = "either device or emulator configuration needs to be provided"
	// MissingDeviceOrEmulator indicates no device or emulator
	MissingDeviceOrSimulator = "either device or simulator configuration needs to be provided"
	// MissingDevice indicates no device provided
	MissingDevice = "no device provided"
	// EmptyAdhocSuiteName is thrown when a flag is specified that has a dependency on the --name flag.
	EmptyAdhocSuiteName = "adhoc suite parameters can only be used with a new adhoc suite by setting --name"
	// UnknownFrameworkConfig indicates unknown framework config
	UnknownFrameworkConfig = "unknown framework configuration"
	// UnableToFetchFrameworkList indicates fail to fetch framework list
	UnableToFetchFrameworkList = "unable to fetch frameworks list"
)

// config settings
const (
	// MissingConfigFile indicates no config file
	MissingConfigFile = "no config file was provided"
	// InvalidSauceConfig indicates it's not a valid sauce config
	InvalidSauceConfig = "invalid sauce config, which is either malformed or corrupt, please refer to https://docs.saucelabs.com/dev/cli/saucectl/#configure-saucectl-for-your-tests for creating a valid config"
)

// common config settings
const (
	// MissingRegion indicates no sauce region provided
	MissingRegion = "no sauce region set"
	// EmptySuite indicates no suites in the config
	EmptySuite = "no suites defined"
	// MissingSuiteName indicates no suite name
	MissingSuiteName = "suite name is not found for suite %d"
	// DuplicateSuiteName indicates duplicate suite name
	DuplicateSuiteName = "suite names must be unique, but found duplicate for '%s'"
	// SuiteNameNotFound indicates it cannot find the specified suite by name
	SuiteNameNotFound = "no suite named '%s' found"
	// InvalidKeyValueInputFormat indicates wrong setting for key-value pairs
	InvalidKeyValueInputFormat = "wrong input format; must be of key-value"
	// InvalidGitRelease indicates the git release is malformed
	InvalidGitRelease = "malformed git release string in metadata"
	// MissingFrameworkVersionConfig indicates empty framework version in the sauce config
	MissingFrameworkVersionConfig = "missing framework version. Check available versions here: https://docs.saucelabs.com/dev/cli/saucectl/#supported-frameworks-and-browsers"
	// UnableToLocateRootDir indicates no rootDir provided
	UnableToLocateRootDir = "unable to locate the rootDir folder %s"
	// UnsupportedBrowser indicates the specified browser is not supported
	UnsupportedBrowser = "browserName: %s is not supported. List of supported browsers: %s"
	// UnsupportedFrameworkVersion indicates the specified framework version is not supported
	UnsupportedFrameworkVersion = "unsupported framework version"
	// InvalidDeviceType indicates invalid device type
	InvalidDeviceType = "deviceType: %s is unsupported for suite: %s. Devices index: %d. Supported device types: %s"
	// MissingDeviceConfig indicates neither device name nor device ID is provided
	MissingDeviceConfig = "missing device name or ID for suite: %s. Devices index: %d"
	// InvalidVisibility indicates that the configured visibility is invalid and has no effect on the test results
	InvalidVisibility = "'%s' is not a valid visibility value. Must be one of [%s]"
	// InvalidLaunchingOption indicates the launching option is invalid
	InvalidLaunchingOption = "illegal launching option '%s', must be %s"
	// NoEmulatorSupport indicates lack of emulator support for the specified region.
	NoEmulatorSupport = "emulators are currently not supported in your specified region"
	// NoFrameworkSupport indicates lack of framework support for the specified region.
	NoFrameworkSupport = "this framework is currently not supported in your specified region"
	// InfoUsingDefaultPlatform indicates using the default value for platform.
	InfoUsingDefaultPlatform = "using default platform '%s' for suite suite '%s'"
	// InvalidReries indicates retries is less than 0
	InvalidReries = "retries should not be less than 0"
	// InvalidPassThreshold indicates invalid pass threshold
	InvalidPassThreshold = "passThreshold should not be greater than retries+1"
	// ShardingConfigurationNoMatchingTests indicates no test matching sharding configuration
	ShardingConfigurationNoMatchingTests = "sharding configuration resulted in no matching tests"
)

// apitesting config settings
const (
	// NoProjectName indicates the absence of a project name in a suite
	NoProjectName = "suites must have a projectName defined"
	// ProjectListFailure indicates failure to get the list of projects
	ProjectListFailure = "unable to list projects"
	// ProjectNotFound indicates the project was not found
	ProjectNotFound = `Project "%s" was not found in region %s`
	// HookQueryFailure indicates failure when fetching the hooks
	HookQueryFailure = "unable to query for hooks"
	// NoHookForProject indicates the absence of available hook
	NoHookForProject = `No hooks found for project "%s"`
	// WebhookCreationLink indicates where to create a webhook.
	WebhookCreationLink = `Go to %s to create a webhook for project "%s"`
	// FailedToPrepareSuites indicates failure of preliminary steps for api-testing
	FailedToPrepareSuites = "failed to get some suites associated hookIDs"
)

// cypress config settings
const (
	// MissingCypressVersion indicates no valid cypress version provided
	MissingCypressVersion = "missing framework version. Check available versions here: https://docs.saucelabs.com/dev/cli/saucectl/#supported-frameworks-and-browsers"
	// IllegalSymbol indicates suitename contains illegal symbol
	IllegalSymbol = "illegal symbol '%c' in suite name: '%s'"
	// MissingBrowserInSuite indicates no browser specified
	MissingBrowserInSuite = "no browser specified in suite '%s'"
	// MissingTestFiles indicates no testFiles specified
	MissingTestFiles = "no test files specified in suite '%s'"
	// UnableToLocateCypressCfg indicates it cannot locate cypress config file by the path
	UnableToLocateCypressCfg = "unable to locate the cypress config file at: %s"
	// InvalidCypressTestingType indicates the testingType should be 'e2e' or 'component'
	InvalidCypressTestingType = "invalid testingType in suite '%s'. testingType should be 'e2e' or 'component' only"
)

// espresso config settings
const (
	// MissingAppPath indicates empty app path
	MissingAppPath = "missing path to app. Define a path to an .apk or .aab file in the espresso.app property of your config"
	// MissingTestAppPath indicates empty testApp path
	MissingTestAppPath = "missing path to test app. Define a path to an .apk or .aab file in the espresso.testApp property of your config"
	// MissingDevicesOrEmulatorConfig indicates no devices or emulator config provided
	MissingDevicesOrEmulatorConfig = "missing devices or emulators configuration for suite: %s"
	// MissingEmulatorName indicates empty emulator name
	MissingEmulatorName = "missing emulator name for suite: %s. Emulators index: %d"
	// InvalidEmulatorName indicates invalid emulator name
	InvalidEmulatorName = `missing "emulator" in emulator name: %s. Suite name: %s. Emulators index: %d`
	// MissingEmulatorPlatformVersion indicates no emulator platform version provided
	MissingEmulatorPlatformVersion = "missing platform versions for emulator: %s. Suite name: %s. Emulators index: %d"
)

// ImageRunner config settings
const (
	// MissingImageRunnerWorkloadType indicates no workload type provided
	MissingImageRunnerWorkloadType = `missing "workload" value for suite: %s`
	// InvalidImageRunnerWorkloadType indicates invalid workload type provided
	InvalidImageRunnerWorkloadType = `%q is an invalid "workload" value for suite: %s`
	// MissingImageRunnerImage indicates no docker image provided
	MissingImageRunnerImage = `missing "image" for suite: %s`
	// ImageRunnerMaxConcurrency indicates that the concurrency value is too high.
	ImageRunnerMaxConcurrency = "Maximum concurrency for imagerunner is 5. Replacing %d with 5."
	// InvalidResourceProfile indicates the resourceProfile is not valid
	InvalidResourceProfile = "invalid resourceProfile for suite: %s, resourceProfile should be of format %v"
	// MissingServiceName indicates no service name provided
	MissingServiceName = `missing "name" for service in suite: %s`
	// MissingServiceImage indicates no docker image provided
	MissingServiceImage = `missing "image" for service: %s in suite: %s`
	// InvalidServiceResourceProfile indicates the service resourceProfile is not valid
	InvalidServiceResourceProfile = "invalid resourceProfile for service: %s in suite: %s, resourceProfile should be of format %v"
)

// testcafe config settings
const (
	// InvalidTestCafeDeviceSetting indicates the unsupported device keyword in the config
	InvalidTestCafeDeviceSetting = "the 'devices' keyword in your config is now reserved for real devices, please use 'simulators' instead"
)

// XCUITest config settings
const (
	// MissingXcuitestAppPath indicates empty app path for xcuitest
	MissingXcuitestAppPath = "missing path to app .ipa"
	// MissingXcuitestTestAppPath indicates empty testApp path for xcuitest
	MissingXcuitestTestAppPath = "missing path to test app .ipa"
	// MissingXcuitestDeviceConfig indicates empty device setting for xcuitest
	MissingXcuitestDeviceConfig = "missing devices configuration for suite: %s"
)

// Smart Retries
const (
	// UnableToFetchFile indicates a failure when fetching a file
	UnableToFetchFile = "Unable to fetch file '%s'"
	// UnableToUnmarshallFile indicates a failure to unmarshall a file
	UnableToUnmarshallFile = "Unable to unmarshall file '%s'"
	// SkippingSmartRetries implies that all tests will be retried.
	SkippingSmartRetries = "Skipping SmartRetry. Retrying all tests from the previous job."
	// RetryWithTests indicates which tests will be run
	RetryWithTests = "Retrying with failed tests: %q"
	// UnableToCreateRunnerConfig indicates a failure to create runner config file
	UnableToCreateRunnerConfig = "Unable to create runner config file"
	// UnableToFilterFailedTests indicates a failure to filter failed tests
	UnableToFilterFailedTests = "Unable to filter failed tests"
	// UnableToArchiveRunnerConfig indicates a failure to archive the runner config file
	UnableToArchiveRunnerConfig = "Unable to archive sauce runner config file"
	// UnableToUploadConfig indicates a failure to upload config
	UnableToUploadConfig = "Unable to upload sauce runner config file %q"
	// UnreliableReport indicates that the job is not smart-retryable due to its
	// status.
	UnreliableReport = "Test reports from errored jobs are not a reliable source to correctly determine failed tests."
)

// container
const (
	// EmptyDockerImgName indicates no docker image name provided
	EmptyDockerImgName = "no docker image specified"
)

// common client
const (
	// InternalServerError indicates internal server error
	InternalServerError = "internal server error"
	// JobNotFound indicates job was not found
	JobNotFound = "job not found"
	// AssetNotFound indicates requested asset was not found
	AssetNotFound = "asset not found"
	// TunnelNotFound indicates tunnel was not found
	TunnelNotFound = "tunnel not found"
	// RetrieveJobHistoryError indicates failed to retrieve job history
	RetrieveJobHistoryError = "Unable to retrieve job history. Launching jobs in the default order."
	// InsightsReportError indicates failure push to insights.
	InsightsReportError = "unable to report result to insights"
)

// npm related messages
const (
	// NpmEmptyURLError indicates a missing URL value
	NpmEmptyURLError = "url of scoped registry at index %d is empty"
	// NpmTooManyDefaultRegistry indicates that there is multiple registry without scope
	NpmTooManyDefaultRegistry = "only one default (scopeless) registry is allowed"
)
