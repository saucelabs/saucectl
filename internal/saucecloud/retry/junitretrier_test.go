package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/saucelabs/saucectl/internal/espresso"
	"github.com/saucelabs/saucectl/internal/job"
	"github.com/saucelabs/saucectl/internal/junit"
	"github.com/saucelabs/saucectl/internal/mocks"
	"github.com/saucelabs/saucectl/internal/xcuitest"
	"github.com/stretchr/testify/assert"
)

func TestAppsRetrier_Retry(t *testing.T) {
	type args struct {
		jobOpts  chan job.StartOptions
		opt      job.StartOptions
		previous job.Job
	}
	type init struct {
		JobService job.Service
		RetryRDC   bool
		RetryVDC   bool
	}
	tests := []struct {
		name     string
		init     init
		args     args
		expected job.StartOptions
	}{
		{
			name: "Job is resent as-it if no RDC",
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					TestOptions: map[string]interface{}{
						"class": []string{"present"},
					},
				},
				previous: job.Job{
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"present"},
				},
			},
		},
		{
			name: "Job is untouched if there is no SmartRetries and is RDC",
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: false,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"present"},
					},
				},
				previous: job.Job{
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"present"},
				},
			},
		},
		{
			name: "Espresso job retrying only failed classes if RDC + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryRDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   espresso.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				Framework:   espresso.Kind,
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			name: "Espresso job retrying only failed classes if RDC + SmartRetry with no orig filters",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryRDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   espresso.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				Framework:   espresso.Kind,
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			name: "XCUITest: Job retrying only failed tests if RDC + SmartRetry with no orig filters",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase name=\"demoTest\" classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryRDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   xcuitest.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					RealDevice: true,
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: true,
				},
			},
			expected: job.StartOptions{
				Framework:   xcuitest.Kind,
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{},
				TestsToRun:  []string{"Demo.Class1/demoTest"},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
				RealDevice: true,
			},
		},
		{
			name: "Job is retrying when VDC + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\">\n        <failure>ERROR</failure>\n    </testcase>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n    <testcase classname=\"Demo.Class3\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			// Regression test: when a VDC Espresso job fails but the JUnit has no <failure>
			// elements (e.g. silent crash), the original class filter must be preserved.
			name: "Espresso: VDC preserves original class filter when JUnit has no failures + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Demo.Class1\"/>\n    <testcase classname=\"Demo.Class2\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   espresso.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				Framework:   espresso.Kind,
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			// Integration test: Cucumber-on-Espresso produces JUnit classnames like
			// "Collect Elements" (display names, not Java classes). These are invalid
			// for Android's "am instrument -e class" filter and must be skipped.
			// The original TestOptions["class"] must be preserved.
			name: "Espresso: VDC skips Cucumber display names and preserves original class filter + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Collect Elements\" name=\"Testing scenario 1\" status=\"success\"/>\n    <testcase classname=\"Collect Elements\" name=\"Testing scenario 2\" status=\"error\">\n        <error>assertion failed</error>\n    </testcase>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   espresso.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"com.example.MyTest"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				Framework:   espresso.Kind,
				DisplayName: "Dummy Test",
				// Original class filter must be preserved when all classnames are non-Java.
				TestOptions: map[string]interface{}{
					"class": []string{"com.example.MyTest"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			// Integration test: mixed JUnit with both valid Java classnames and
			// Cucumber display names. Only valid Java classes should be retried.
			name: "Espresso: VDC retries only valid Java classes when mixed with Cucumber display names + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase classname=\"Collect Elements\" name=\"Testing scenario 1\" status=\"error\">\n        <error>assertion failed</error>\n    </testcase>\n    <testcase classname=\"com.example.RealTest\" name=\"testLogin\" status=\"error\">\n        <error>login failed</error>\n    </testcase>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   espresso.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				Framework:   espresso.Kind,
				DisplayName: "Dummy Test",
				// Only the valid Java class is retried; Cucumber display name is skipped.
				TestOptions: map[string]interface{}{
					"class": []string{"com.example.RealTest#testLogin"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			name: "XCUITest: VDC retries only failed tests when JUnit has failures + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase name=\"testLogin\" classname=\"SampleApp.AuthTests\">\n        <failure>Test failed</failure>\n    </testcase>\n    <testcase name=\"testLogout\" classname=\"SampleApp.AuthTests\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   xcuitest.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"SampleApp/AuthTests/testLogin"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				Framework:   xcuitest.Kind,
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"SampleApp/AuthTests/testLogin"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			// Regression test: when a VDC XCUITest job fails but the JUnit has no <failure>
			// elements (e.g. silent crash), the original class filter must be preserved.
			// Previously, setClassesToRetry unconditionally overwrote opt.TestOptions["class"]
			// with an empty slice, causing the VDC to run the entire test bundle without filters.
			name: "XCUITest: VDC preserves original class filter when JUnit has no failures + SmartRetry",
			init: init{
				JobService: &mocks.FakeJobService{
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							// Valid JUnit with test cases but no <failure> or <error> elements.
							return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n<testsuite>\n    <testcase name=\"testCheckout\" classname=\"SampleApp.CartTests\"/>\n</testsuite>\n"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					Framework:   xcuitest.Kind,
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"SampleApp/CartTests/testCheckout"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				Framework:   xcuitest.Kind,
				DisplayName: "Dummy Test",
				// The original single-test filter must survive the retry unchanged.
				TestOptions: map[string]interface{}{
					"class": []string{"SampleApp/CartTests/testCheckout"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			name: "Base Retry if junit is malformed",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("malformed"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
		{
			name: "Base Retry if getting junit.xml is failing",
			init: init{
				JobService: &mocks.FakeJobService{
					ReadJobFn:              nil,
					PollJobFn:              nil,
					GetJobAssetFileNamesFn: nil,
					GetJobAssetFileContentFn: func(_ context.Context, jobID, fileName string) ([]byte, error) {
						if jobID == "fake-job-id" && fileName == junit.FileName {
							return []byte("malformed"), nil
						}
						return []byte{}, errors.New("unknown file")
					},
				},
				RetryVDC: true,
			},
			args: args{
				jobOpts: make(chan job.StartOptions),
				opt: job.StartOptions{
					DisplayName: "Dummy Test",
					SmartRetry: job.SmartRetry{
						FailedOnly: true,
					},
					TestOptions: map[string]interface{}{
						"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
					},
				},
				previous: job.Job{
					ID:    "fake-buggy-job-id",
					IsRDC: false,
				},
			},
			expected: job.StartOptions{
				DisplayName: "Dummy Test",
				TestOptions: map[string]interface{}{
					"class": []string{"Demo.Class1", "Demo.Class2", "Demo.Class3"},
				},
				SmartRetry: job.SmartRetry{
					FailedOnly: true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &JunitRetrier{
				JobService: tt.init.JobService,
			}
			go b.Retry(context.Background(), tt.args.jobOpts, tt.args.opt, tt.args.previous)
			newOpt := <-tt.args.jobOpts
			assert.Equal(t, tt.expected, newOpt)
		})
	}
}

func Test_normalizeXCUITestClassName(t *testing.T) {
	type args struct {
		name string
		rdc  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "dot separated is not VMD conform",
			args: args{
				name: "DemoAppTests.ClassyTest",
				rdc:  false,
			},
			want: "DemoAppTests/ClassyTest",
		},
		{
			name: "already VMD conform with slashes",
			args: args{
				name: "DemoAppTests/ClassyTest",
				rdc:  false,
			},
			want: "DemoAppTests/ClassyTest",
		},
		{
			name: "already VMD conform without separators",
			args: args{
				name: "DemoAppTests",
				rdc:  false,
			},
			want: "DemoAppTests",
		},
		{
			name: "already RDC conform",
			args: args{
				name: "DemoAppTests.ClassyTest",
				rdc:  true,
			},
			want: "DemoAppTests.ClassyTest",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, conformXCUITestClassName(tt.args.name, tt.args.rdc), "conformXCUITestClassName(%v)", tt.args.name)
		})
	}
}

func Test_isJavaClassName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Valid Java classnames
		{name: "fully qualified class", input: "com.example.MyTest", want: true},
		{name: "nested class with dollar", input: "com.example.MyTest$Inner", want: true},
		{name: "two segments", input: "Demo.Class1", want: true},
		{name: "underscore in package", input: "com.my_app.Test", want: true},
		{name: "dollar in class", input: "com.example.$Generated", want: true},

		// Cucumber display names
		{name: "cucumber feature name with spaces", input: "Collect Elements", want: false},
		{name: "cucumber long scenario name", input: "Testing Collect Elements in different scenarios", want: false},

		// Edge cases - no dot
		{name: "single word no package", input: "MyTest", want: false},
		{name: "empty string", input: "", want: false},

		// Edge cases - dot present but not valid Java
		{name: "cucumber name with dot", input: "Collect.Elements v2", want: false},
		{name: "version string", input: "Login.v2.0", want: false},
		{name: "numeric segment", input: "123.456", want: false},
		{name: "leading dot", input: ".com.example", want: false},
		{name: "trailing dot", input: "com.example.", want: false},
		{name: "consecutive dots", input: "com..example", want: false},
		{name: "special characters", input: "com.example/MyTest", want: false},
		{name: "hyphen in segment", input: "com.my-app.Test", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isJavaClassName(tt.input))
		})
	}
}
