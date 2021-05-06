package junit

import "encoding/xml"

type TestCase struct {
	Name       string `xml:"name,attr"`
	Assertions string `xml:"assertions,attr"`
	Time       string `xml:"time,attr"`
	ClassName  string `xml:"classname,attr"`
	Status     string `xml:"status,attr"`
	SystemOut  string `xml:"system-out"`
	Error      string `xml:"error"`
	Failure    string `xml:"failure"`
}

type TestSuite struct {
	Name      string     `xml:"name,attr"`
	Tests     string     `xml:"tests,attr"`
	Errors    string     `xml:"errors,attr,omitempty"`
	Failures  string     `xml:"failures,attr,omitempty"`
	Time      string     `xml:"time,attr,omitempty"`
	Disabled  string     `xml:"disabled,attr,omitempty"`
	Skipped   string     `xml:"skipped,attr,omitempty"`
	Timestamp string     `xml:"timestamp,attr,omitempty"`
	Package   string     `xml:"package,attr,omitempty"`
	TestCase  []TestCase `xml:"testcase"`
	SystemOut string     `xml:"system-out"`
}

type TestSuites struct {
	XMLName   xml.Name    `xml:"testsuites"`
	TestSuite []TestSuite `xml:"testsuite"`
	Name      string      `xml:"name,attr,omitempty"`
	Time      string      `xml:"time,attr,omitempty"`
	Tests     string      `xml:"tests,attr,omitempty"`
	Failures  string      `xml:"failures,attr,omitempty"`
	Disabled  string      `xml:"disabled,attr,omitempty"`
	Errors    string      `xml:"errors,attr,omitempty"`
}

func Parse(data []byte) TestSuites {
	tss := TestSuites{}
	err := xml.Unmarshal(data, &tss)
	// root <testsuites> is optional
	if err != nil {
		ts := TestSuite{}
		err = xml.Unmarshal(data, &ts)
		if err == nil {
			return TestSuites{
				TestSuite: []TestSuite{ts},
			}
		}
	}
	return tss
}
