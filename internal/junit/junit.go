package junit

import (
	"encoding/xml"
	"fmt"
)

// FileName is the name of the JUnit report.
const FileName = "junit.xml"

// TestCase maps to <testcase> element
type TestCase struct {
	Name string `xml:"name,attr"`
	// Assertions is the number of assertions in the test case.
	Assertions string `xml:"assertions,attr,omitempty"`
	// Time in seconds it took to run the test case.
	Time string `xml:"time,attr"`
	// Timestamp as specified by ISO 8601 (2014-01-21T16:17:18).
	// Timezone is optional.
	Timestamp string `xml:"timestamp,attr"`
	ClassName string `xml:"classname,attr"`
	// Status indicates success or failure of the test. May be used instead of
	// Error, Failure or Skipped or in addition to them.
	Status    string   `xml:"status,attr,omitempty"`
	File      string   `xml:"file,attr,omitempty"`
	SystemErr string   `xml:"system-err,omitempty"`
	SystemOut string   `xml:"system-out,omitempty"`
	Error     *Error   `xml:"error,omitempty"`
	Failure   *Failure `xml:"failure,omitempty"`
	Skipped   *Skipped `xml:"skipped,omitempty"`
}

// Failure maps to either a <failure> or <error> element. It usually indicates
// assertion failures. Depending on the framework, this may also indicate an
// unexpected error, much like Error does. Some frameworks use Error or the
// 'status' attribute on TestCase instead.
type Failure struct {
	// Message is a short description of the failure.
	Message string `xml:"message,attr"`
	// Type is the type of failure, e.g. "java.lang.AssertionError".
	Type string `xml:"type,attr"`
	// Text is a failure description or stack trace.
	Text string `xml:",chardata"`
}

// Error maps to <error> element. It usually indicates unexpected errors.
// Some frameworks use Failure or the 'status' attribute on TestCase instead.
type Error struct {
	// Message is a short description of the error.
	Message string `xml:"message,attr"`
	// Type is the type of error, e.g. "java.lang.NullPointerException".
	Type string `xml:"type,attr"`
	// Text is an error description or stack trace.
	Text string `xml:",chardata"`
}

// Skipped maps to <skipped> element. Indicates a skipped test. Some frameworks
// use the 'status' attribute on TestCase instead.
type Skipped struct {
	// Message is a short description that explains why the test was skipped.
	Message string `xml:"message,attr"`
}

// TestSuite maps to <testsuite> element
type TestSuite struct {
	Name       string     `xml:"name,attr"`
	Tests      int        `xml:"tests,attr"`
	Properties []Property `xml:"properties>property"`
	Errors     int        `xml:"errors,attr,omitempty"`
	Failures   int        `xml:"failures,attr,omitempty"`
	// Disabled is the number of disabled or skipped tests. Some frameworks use Skipped instead.
	Disabled int `xml:"disabled,attr,omitempty"`
	// Skipped is the number of skipped or disabled tests. Some frameworks use Disabled instead.
	Skipped int `xml:"skipped,attr,omitempty"`
	// Time in seconds it took to run the test suite.
	Time string `xml:"time,attr,omitempty"`
	// Timestamp as specified by ISO 8601 (2014-01-21T16:17:18). Timezone may not be specified.
	Timestamp string     `xml:"timestamp,attr,omitempty"`
	Package   string     `xml:"package,attr,omitempty"`
	File      string     `xml:"file,attr,omitempty"`
	TestCases []TestCase `xml:"testcase"`
	SystemErr string     `xml:"system-err,omitempty"`
	SystemOut string     `xml:"system-out,omitempty"`
}

// TestSuites maps to root junit <testsuites> element
type TestSuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	TestSuites []TestSuite `xml:"testsuite"`
	Name       string      `xml:"name,attr,omitempty"`
	// Time in seconds it took to run all the test suites.
	Time  string `xml:"time,attr,omitempty"`
	Tests int    `xml:"tests,attr,omitempty"`
	// Disabled is the number of disabled or skipped tests. Some frameworks use Skipped instead.
	Disabled int `xml:"disabled,attr,omitempty"`
	// Skipped is the number of skipped or disabled tests. Some frameworks use Disabled instead.
	Skipped  int `xml:"skipped,attr,omitempty"`
	Failures int `xml:"failures,attr,omitempty"`
	Errors   int `xml:"errors,attr,omitempty"`
}

// Property maps to a <property> element that's part of <properties>.
type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Parse a junit report from an XML encoded byte string. The root <testsuites>
// element is optional if there's only one <testsuite> element. In that case,
// Parse will parse the <testsuite> and wrap it in a TestSuites struct.
func Parse(data []byte) (TestSuites, error) {
	var tss TestSuites
	err := xml.Unmarshal(data, &tss)
	if err != nil {
		// root <testsuites> is optional
		// parse a <testsuite> element instead
		var ts TestSuite
		if err = xml.Unmarshal(data, &ts); err != nil {
			return tss, err
		}

		tss.TestSuites = []TestSuite{
			ts,
		}
	}

	return tss, err
}

// GetFailedXCUITests get failed XCUITest test list from testcases.
func GetFailedXCUITests(testCases []TestCase) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			// The format of the filtered test is "<className>/<testMethodName>".
			// Fallback to <className> if the test method name is unexpectedly empty.
			// tc.Name: <testMethodName>
			// tc.ClassName: <className>
			if tc.Name != "" {
				classes[fmt.Sprintf("%s/%s", tc.ClassName, tc.Name)] = true
			} else {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

// GetFailedEspressoTests get failed espresso test list from testcases.
func GetFailedEspressoTests(testCases []TestCase) []string {
	classes := map[string]bool{}
	for _, tc := range testCases {
		if tc.Error != nil || tc.Failure != nil {
			// The format of the filtered test is "<className>#<testMethodName>".
			// Fallback to <className> if the test method name is unexpectedly empty.
			// tc.Name: <testMethodName>
			// tc.ClassName: <className>
			if tc.Name != "" {
				classes[fmt.Sprintf("%s#%s", tc.ClassName, tc.Name)] = true
			} else {
				classes[tc.ClassName] = true
			}
		}
	}
	return getKeysFromMap(classes)
}

// CollectTestCases collects testcases from a report.
func CollectTestCases(testsuites TestSuites) []TestCase {
	var tc []TestCase
	for _, s := range testsuites.TestSuites {
		tc = append(tc, s.TestCases...)
	}
	return tc
}

func getKeysFromMap(mp map[string]bool) []string {
	var keys = make([]string, len(mp))
	var i int
	for k := range mp {
		keys[i] = k
		i++
	}
	return keys
}
