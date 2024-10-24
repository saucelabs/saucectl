package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

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

func TestWhen_IsNow(t *testing.T) {
	type args struct {
		passed bool
	}
	tests := []struct {
		name string
		w    When
		args args
		want bool
	}{
		{
			name: "undefined means never",
			w:    "",
			args: args{
				passed: true,
			},
			want: false,
		},
		{
			name: "never",
			w:    WhenNever,
			args: args{
				passed: false,
			},
			want: false,
		},
		{
			name: "never means never",
			w:    WhenNever,
			args: args{
				passed: true,
			},
			want: false,
		},
		{
			name: "always",
			w:    WhenAlways,
			args: args{
				passed: true,
			},
			want: true,
		},
		{
			name: "always, even if something failed",
			w:    WhenAlways,
			args: args{
				passed: false,
			},
			want: true,
		},
		{
			name: "on failure",
			w:    WhenFail,
			args: args{
				passed: false,
			},
			want: true,
		},
		{
			name: "only on failure",
			w:    WhenFail,
			args: args{
				passed: true,
			},
			want: false,
		},
		{
			name: "on success",
			w:    WhenPass,
			args: args{
				passed: true,
			},
			want: true,
		},
		{
			name: "only on success",
			w:    WhenPass,
			args: args{
				passed: false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, tt.w.IsNow(tt.args.passed), "IsNow(%v)", tt.args.passed)
		})
	}
}

func TestNpm_SetDefaults(t *testing.T) {
	type fields struct {
		Registry         string
		Registries       []Registry
		Framework        string
		FrameworkVersion string
	}
	tests := []struct {
		name   string
		fields fields
		want   []Registry
	}{
		{
			name: "Only one registry",
			fields: fields{
				Registries:       []Registry{{URL: "http://npmjs.org"}},
				Framework:        "dummy",
				FrameworkVersion: "",
			},
			want: []Registry{
				{URL: "http://npmjs.org"},
			},
		},
		{
			name: "Only legacy registry",
			fields: fields{
				Registry:         "http://npmjs.org",
				Framework:        "dummy",
				FrameworkVersion: "",
			},
			want: []Registry{
				{URL: "http://npmjs.org"},
			},
		},
		{
			name: "Legacy registry + Newer",
			fields: fields{
				Registry: "http://npmjs.org",
				Registries: []Registry{
					{URL: "http://npmjs-2.org"},
				},
				Framework:        "dummy",
				FrameworkVersion: "",
			},
			want: []Registry{
				{URL: "http://npmjs-2.org"},
				{URL: "http://npmjs.org"},
			},
		},
		{
			name: "Do not migrate older versions",
			fields: fields{
				Registry:         "http://npmjs.org",
				Registries:       []Registry{},
				Framework:        "cypress",
				FrameworkVersion: "12.14.0",
			},
			want: []Registry{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			n := &Npm{
				Registry:   tt.fields.Registry,
				Registries: tt.fields.Registries,
			}
			n.SetDefaults(tt.fields.Framework, tt.fields.FrameworkVersion)
		})
	}
}

func TestValidateRegistries(t *testing.T) {
	tests := []struct {
		name    string
		args    []Registry
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "passing empty",
			args: []Registry{},
			wantErr: func(_ assert.TestingT, err error, _ ...interface{}) bool {
				return err == nil
			},
		},
		{
			name: "passing with registry",
			args: []Registry{
				{URL: "http://npmjs.org"},
			},
			wantErr: func(_ assert.TestingT, err error, _ ...interface{}) bool {
				return err == nil
			},
		},
		{
			name: "passing with registry + scoped",
			args: []Registry{
				{URL: "http://npmjs.org"},
				{URL: "http://npmjs-2.org", Scope: "@scoped"},
			},
			wantErr: func(_ assert.TestingT, err error, _ ...interface{}) bool {
				return err == nil
			},
		},
		{
			name: "failing with multiple default",
			args: []Registry{
				{URL: "http://npmjs.org"},
				{URL: "http://npmjs-2.org"},
			},
			wantErr: func(_ assert.TestingT, err error, _ ...interface{}) bool {
				return err != nil && err.Error() == "too many registries (2) are without scope"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, ValidateRegistries(tt.args), fmt.Sprintf("ValidateRegistries(%v)", tt.args))
		})
	}
}
