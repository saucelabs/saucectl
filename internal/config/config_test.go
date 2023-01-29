package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	assert2 "gotest.tools/v3/assert"

	"github.com/stretchr/testify/assert"
)

func TestStandardizeVersionFormat(t *testing.T) {
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("v5.6.0"))
	assert.Equal(t, "5.6.0", StandardizeVersionFormat("5.6.0"))
}

func TestCleanNpmPackages(t *testing.T) {
	packages := map[string]string{
		"cypress": "7.7.0",
		"mocha":   "1.2.3",
	}

	cleaned := CleanNpmPackages(packages, []string{"cypress"})
	assert.NotContains(t, cleaned, "cypress")
	assert.Contains(t, cleaned, "mocha")

	packages = map[string]string{}

	cleaned = CleanNpmPackages(packages, []string{"somepackage"})
	assert.NotNil(t, cleaned)
	assert.Len(t, cleaned, 0)

	packages = nil
	cleaned = CleanNpmPackages(packages, []string{})
	assert.Nil(t, cleaned)
}

func TestConfig_ExpandEnv(t *testing.T) {
	envMap := map[string]string{
		"REGION":                      "us-west-1",
		"TAG1":                        "my_tag1",
		"TAG2":                        "my_tag2",
		"BUILD":                       "my_build",
		"TUNNEL_ID":                   "my_tunnel_id",
		"APP":                         "espresso_app",
		"OTHER_APP1":                  "espresso_other_app1",
		"NOT_CLASS1":                  "not_class1",
		"NOT_CLASS2":                  "not_class2",
		"CLASS1":                      "test_class1",
		"CLASS2":                      "test_class2",
		"PACKAGE":                     "my_package",
		"SUITE_NAME":                  "my_suite_name",
		"TIMEOUT":                     "10s",
		"DEVICE_ID":                   "my_device_id",
		"ARTIFACT_MATCH1":             "artifact_match1",
		"ARTIFACT_MATCH2":             "artifact_match2",
		"ARTIFACT_WHEN":               "always",
		"NOTIFICATION_SLACK_CHANNEL1": "channel1",
		"NOTIFICATION_SLACK_CHANNEL2": "channel2",
		"NOTIFICATION_SLACK_SEND":     "always",
	}

	for key, val := range envMap {
		os.Setenv(key, val)
	}

	testObj := map[string]interface{}{
		"sauce": map[string]interface{}{
			"region":   "$REGION",
			"metadata": map[string]interface{}{},
			"tunnel": map[string]interface{}{
				"id": "$TUNNEL_ID",
			},
		},
		"espresso": map[string]interface{}{
			"app":       "$APP",
			"otherApps": []interface{}{"$OTHER_APP1"},
		},
		"suites": []interface{}{
			map[string]interface{}{
				"name": "$SUITE_NAME",
				"devices": []interface{}{
					map[string]interface{}{
						"id": "$DEVICE_ID",
					},
				},
				"testOptions": map[string]interface{}{
					"notClass": []interface{}{"$NOT_CLASS1", "$NOT_CLASS2"},
					"class":    []interface{}{"$CLASS1", "$CLASS2"},
					"package":  "$PACKAGE",
				},
				"timeout": "$TIMEOUT",
			},
		},
		"artifacts": map[string]interface{}{
			"download": map[string]interface{}{
				"match": []interface{}{"$ARTIFACT_MATCH1", "$ARTIFACT_MATCH2"},
				"when":  "$ARTIFACT_WHEN",
			},
		},
		"notifications": map[string]interface{}{
			"slack": map[string]interface{}{
				"channels": []interface{}{"$NOTIFICATION_SLACK_CHANNEL1", "$NOTIFICATION_SLACK_CHANNEL2"},
				"send":     "$NOTIFICATION_SLACK_SEND",
			},
		},
	}

	expectObj := map[string]interface{}{
		"sauce": map[string]interface{}{
			"region":   "us-west-1",
			"metadata": map[string]interface{}{},
			"tunnel": map[string]interface{}{
				"id": "my_tunnel_id",
			},
		},
		"espresso": map[string]interface{}{
			"app":       "espresso_app",
			"otherApps": []interface{}{"espresso_other_app1"},
		},
		"suites": []interface{}{
			map[string]interface{}{
				"name": "my_suite_name",
				"devices": []interface{}{
					map[string]interface{}{
						"id": "my_device_id",
					},
				},
				"testOptions": map[string]interface{}{
					"notClass": []interface{}{"not_class1", "not_class2"},
					"class":    []interface{}{"test_class1", "test_class2"},
					"package":  "my_package",
				},
				"timeout": "10s",
			},
		},
		"artifacts": map[string]interface{}{
			"download": map[string]interface{}{
				"match": []interface{}{"artifact_match1", "artifact_match2"},
				"when":  "always",
			},
		},
		"notifications": map[string]interface{}{
			"slack": map[string]interface{}{
				"channels": []interface{}{"channel1", "channel2"},
				"send":     "always",
			},
		},
	}

	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "Test espresso config",
			input:    testObj,
			expected: expectObj,
		},
		{
			name:     "Test empty config",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name:     "Test nil",
			input:    nil,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := expandEnv(tc.input)
			assert.False(t, strings.Contains(fmt.Sprint(result), "$"))
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestShouldDownloadArtifact(t *testing.T) {
	type testCase struct {
		name     string
		config   ArtifactDownload
		jobID    string
		passed   bool
		timedOut bool
		async    bool
		want     bool
	}
	testCases := []testCase{
		{
			name:   "should not download when jobID is empty even being required",
			config: ArtifactDownload{When: WhenAlways},
			jobID:  "",
			want:   false,
		},
		{
			name:   "should not download when jobID is empty and not being required",
			config: ArtifactDownload{When: WhenNever},
			jobID:  "",
			want:   false,
		},
		{
			name:   "should not download when jobs are processed asynchronously",
			config: ArtifactDownload{When: WhenAlways},
			jobID:  "fake-id",
			async:  true,
			want:   false,
		},
		{
			name:   "should download artifacts when it's always required",
			config: ArtifactDownload{When: WhenAlways},
			jobID:  "fake-id",
			want:   true,
		},
		{
			name:   "should download artifacts when it's always required even it's failed",
			config: ArtifactDownload{When: WhenAlways},
			jobID:  "fake-id",
			passed: false,
			want:   true,
		},
		{
			name:   "should not download artifacts when it's not required",
			config: ArtifactDownload{When: WhenNever},
			jobID:  "fake-id",
			passed: true,
			want:   false,
		},
		{
			name:   "should not download artifacts when it's not required and failed",
			config: ArtifactDownload{When: WhenNever},
			jobID:  "fake-id",
			passed: false,
			want:   false,
		},
		{
			name:   "should download artifacts when it only requires passed one and test is passed",
			config: ArtifactDownload{When: WhenPass},
			jobID:  "fake-id",
			passed: true,
			want:   true,
		},
		{
			name:   "should download artifacts when it requires passed one but test is failed",
			config: ArtifactDownload{When: WhenPass},
			jobID:  "fake-id",
			passed: false,
			want:   false,
		},
		{
			name:   "should download artifacts when it requirs failed one but test is passed",
			config: ArtifactDownload{When: WhenFail},
			jobID:  "fake-id",
			passed: true,
			want:   false,
		},
		{
			name:   "should download artifacts when it requires failed one and test is failed",
			config: ArtifactDownload{When: WhenFail},
			jobID:  "fake-id",
			passed: false,
			want:   true,
		},
		{
			name:     "should not download artifacts when it has timedOut",
			config:   ArtifactDownload{When: WhenFail},
			jobID:    "fake-id",
			passed:   false,
			timedOut: true,
			want:     false,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldDownloadArtifact(tt.jobID, tt.passed, tt.timedOut, tt.async, tt.config)
			assert2.Equal(t, tt.want, got)
		})
	}
}
