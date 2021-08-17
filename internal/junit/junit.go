package junit

import "encoding/xml"

// TestCase maps to <testcase> element
type TestCase struct {
	Name       string `xml:"name,attr"`
	Assertions string `xml:"assertions,attr,omitempty"`
	Time       string `xml:"time,attr"`
	ClassName  string `xml:"classname,attr"`
	Status     string `xml:"status,attr,omitempty"`
	SystemOut  string `xml:"system-out,omitempty"`
	Error      string `xml:"error,omitempty"`
	Failure    string `xml:"failure,omitempty"`
}

// TestSuite maps to <testsuite> element
type TestSuite struct {
	Name       string     `xml:"name,attr"`
	Tests      int        `xml:"tests,attr"`
	Properties []Property `xml:"properties>property"`
	Errors     int        `xml:"errors,attr,omitempty"`
	Failures   int        `xml:"failures,attr,omitempty"`
	Disabled   int        `xml:"disabled,attr,omitempty"`
	Skipped    int        `xml:"skipped,attr,omitempty"`
	Time       string     `xml:"time,attr,omitempty"`
	Timestamp  string     `xml:"timestamp,attr,omitempty"`
	Package    string     `xml:"package,attr,omitempty"`
	TestCases  []TestCase `xml:"testcase"`
	SystemOut  string     `xml:"system-out,omitempty"`
}

// TestSuites maps to root junit <testsuites> element
type TestSuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	TestSuites []TestSuite `xml:"testsuite"`
	Name       string      `xml:"name,attr,omitempty"`
	Time       string      `xml:"time,attr,omitempty"`
	Tests      int         `xml:"tests,attr,omitempty"`
	Failures   int         `xml:"failures,attr,omitempty"`
	Disabled   int         `xml:"disabled,attr,omitempty"`
	Errors     int         `xml:"errors,attr,omitempty"`
}

// Property maps to a <property> element that's part of <properties>.
type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Parse parses an xml-encoded byte string and returns a `TestSuites` struct
// The root <testsuites> element is optional so if its missing, Parse will parse a <testsuite> and wrap it in a `TestSuites` struct
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
