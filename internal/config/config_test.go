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
		"TUNNEL_NAME":                 "my_tunnel_name",
		"APP":                         "espresso_app",
		"TEST_APP":                    "espresso_test_app",
		"OTHER_APP1":                  "espresso_other_app1",
		"NOT_CLASS1":                  "not_class1",
		"NOT_CLASS2":                  "not_class2",
		"CLASS1":                      "test_class1",
		"CLASS2":                      "test_class2",
		"PACKAGE":                     "my_package",
		"NOT_PACKAGE":                 "not_package",
		"SIZE":                        "my_size",
		"ANNOTATION":                  "my_annotation",
		"NOT_ANNOTATION":              "not_annotation",
		"NUMSHARDS":                   "3",
		"SUITE_NAME":                  "my_suite_name",
		"TIMEOUT":                     "10s",
		"DEVICE_ID":                   "my_device_id",
		"DEVICE_NAME":                 "my_device_name",
		"PLATFORMNAME1":               "my_platform_name1",
		"PLATFORMNAME2":               "my_platform_name2",
		"ORIENTATION":                 "landscape",
		"PLATFORM_VERSION1":           "platform_version1",
		"PLATFORM_VERSION2":           "platform_version2",
		"ARTIFACT_MATCH1":             "artifact_match1",
		"ARTIFACT_MATCH2":             "artifact_match2",
		"ARTIFACT_WHEN":               "always",
		"ARTIFACT_DIRECTORY":          "artifact_directory",
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
				"id":   "$TUNNEL_ID",
				"name": "$TUNNEL_NAME",
			},
		},
		"espresso": map[string]interface{}{
			"app":       "$APP",
			"testApp":   "$TEST_APP",
			"otherApps": []interface{}{"$OTHER_APP1"},
		},
		"suites": []interface{}{
			map[string]interface{}{
				"name": "$SUITE_NAME",
				"devices": []interface{}{
					map[string]interface{}{
						"id":              "$DEVICE_ID",
						"name":            "$DEVICE_NAME",
						"platformName":    "$PLATFORMNAME1",
						"platformVersion": "$PLATFORM_VERSION1",
					},
				},
				"testOptions": map[string]interface{}{
					"notClass":     []interface{}{"$NOT_CLASS1", "$NOT_CLASS2"},
					"class":        []interface{}{"$CLASS1", "$CLASS2"},
					"package":      "$PACKAGE",
					"notPackage":   "$NOT_PACKAGE",
					"size":         "$SIZE",
					"annotation":   "$ANNOTATION",
					"notAnotation": "$NOT_ANNOTATION",
					"numShards":    "$NUMSHARDS",
				},
				"timeout": "$TIMEOUT",
			},
		},
		"artifacts": map[string]interface{}{
			"download": map[string]interface{}{
				"match":     []interface{}{"$ARTIFACT_MATCH1", "$ARTIFACT_MATCH2"},
				"when":      "$ARTIFACT_WHEN",
				"directory": "$ARTIFACT_DIRECTORY",
			},
		},
		"notifications": map[string]interface{}{
			"slack": map[string]interface{}{
				"channels": []interface{}{"$NOTIFICATION_SLACK_CHANNEL1", "$NOTIFICATION_SLACK_CHANNEL2"},
				"send":     "$NOTIFICATION_SLACK_SEND",
			},
		},
	}

	result := expandEnv(testObj)
	assert.False(t, strings.Contains(fmt.Sprint(result), "$"))

	re := result.(map[string]interface{})
	for k1, v1 := range re {
		switch k1 {
		case "sauce":
			v := v1.(map[string]interface{})
			assert.Equal(t, v["region"], "us-west-1")
			for k2, v2 := range v["tunnel"].(map[string]interface{}) {
				if k2 == "id" {
					assert.Equal(t, "my_tunnel_id", v2)
				}
				if k2 == "name" {
					assert.Equal(t, "my_tunnel_name", v2)
				}
			}
		case "espresso":
			v := v1.(map[string]interface{})
			assert.Equal(t, "espresso_app", v["app"])
			assert.Equal(t, "espresso_test_app", v["testApp"])
			assert.Equal(t, []interface{}{"espresso_other_app1"}, v["otherApps"])
		case "suites":
			v := v1.([]interface{})[0].(map[string]interface{})
			assert.Equal(t, "my_suite_name", v["name"])
			for k2, v2 := range v {
				if k2 == "devices" {
					device := v2.([]interface{})[0].(map[string]interface{})
					for k3, v3 := range device {
						if k3 == "id" {
							assert.Equal(t, "my_device_id", v3)
						}
						if k3 == "name" {
							assert.Equal(t, "my_device_name", v3)
						}
						if k3 == "platformName" {
							assert.Equal(t, "my_platform_name1", v3)
						}
						if k3 == "platformVersion" {
							assert.Equal(t, "platform_version1", v3)
						}
					}
				}
				if k2 == "testOptions" {
					v := v2.(map[string]interface{})
					for k3, v3 := range v {
						if k3 == "notClass" {
							assert.Equal(t, []interface{}{"not_class1", "not_class2"}, v3)
						}
						if k3 == "class" {
							assert.Equal(t, []interface{}{"test_class1", "test_class2"}, v3)
						}
						if k3 == "package" {
							assert.Equal(t, "my_package", v3)
						}
						if k3 == "notPackage" {
							assert.Equal(t, "not_package", v3)
						}
						if k3 == "size" {
							assert.Equal(t, "my_size", v3)
						}
						if k3 == "annotation" {
							assert.Equal(t, "my_annotation", v3)
						}
						if k3 == "notAnnotation" {
							assert.Equal(t, "not_annotation", v3)
						}
						if k3 == "numShards" {
							assert.Equal(t, "3", v3)
						}
					}
				}
				if k2 == "timeout" {
					assert.Equal(t, "10s", v2)
				}
			}
		case "artifacts":
			v := v1.(map[string]interface{})
			for k2, v2 := range v {
				if k2 == "download" {
					for k3, v3 := range v2.(map[string]interface{}) {
						if k3 == "match" {
							assert.Equal(t, []interface{}{"artifact_match1", "artifact_match2"}, v3)
						}
						if k3 == "when" {
							assert.Equal(t, "always", v3)
						}
						if k3 == "directory" {
							assert.Equal(t, "artifact_directory", v3)
						}
					}
				}
			}
		case "notifications":
			v := v1.(map[string]interface{})
			for k2, v2 := range v {
				if k2 == "slack" {
					for k3, v3 := range v2.(map[string]interface{}) {
						if k3 == "channels" {
							assert.Equal(t, []interface{}{"channel1", "channel2"}, v3)
						}
						if k3 == "send" {
							assert.Equal(t, "always", v3)
						}
					}
				}
			}
		}
	}
}
