package mocks

import "strings"

type FakeFrameworkInfoReader struct {}

func (fir *FakeFrameworkInfoReader) Frameworks() ([]string, error) {
	return []string{"Cypress", "Espresso", "Playwright", "Puppeteer", "TestCafe", "XCUITest"}, nil
}

func (fir *FakeFrameworkInfoReader) Versions(frameworkName, region string) ([]string, error) {
	switch strings.ToLower(frameworkName) {
	case "cypress":
		return []string{"7.6.0", "7.5.0", "6.5.0", "5.3.0"}, nil
	default:
		return []string{"1.0.0", "0.9.0", "0.8.0"}, nil
	}
}

func (fir *FakeFrameworkInfoReader) Platforms(frameworkName, region, frameworkVersion string) ([]string, error) {
	switch strings.ToLower(frameworkName) {
	case "xcuitest":
		return []string{"iOS 14.3"}, nil
	case "espresso":
		return []string{"Android"}, nil
	case "testcafe":
		return []string{"Windows 10", "macOS 11.0", "docker"}, nil
	default:
		return []string{"Windows 10", "docker"}, nil
	}
}

func (fir *FakeFrameworkInfoReader) Browsers(frameworkName, region, frameworkVersion, platformName string) ([]string, error) {
	switch strings.ToLower(frameworkName) {
	case "xcuitest":
	case "espresso":
		return []string{}, nil
	case "testcafe":
		return []string{"chrome", "firefox", "safari"}, nil
	case "playwright":
		return []string{"chromium", "firefox", "webkit"}, nil
	default:
		return []string{"chrome", "firefox"}, nil
	}
	return []string{}, nil
}