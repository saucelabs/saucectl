package msg

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// SignupMessage explains how to obtain a Sauce Labs account and where to find the access key.
const SignupMessage = `Don't have an account? Signup here:
https://bit.ly/saucectl-signup

Already have an account? Get your username and access key here:
https://app.saucelabs.com/user-settings`

// SauceIgnoreNotExist is a recommendation to create a .sauceignore file in the case that it is missing.
const SauceIgnoreNotExist = `The .sauceignore file does not exist. We *highly* recommend creating one so that saucectl does not
create archives with unnecessary files. You are very likely to experience longer startup times.

For more information, visit https://docs.saucelabs.com/dev/cli/saucectl/usage/use-cases/#excluding-files-from-the-bundle

or peruse some of our example repositories:
  - https://github.com/saucelabs/saucectl-cypress-example
  - https://github.com/saucelabs/saucectl-playwright-example
  - https://github.com/saucelabs/saucectl-testcafe-example`

// SauceIgnoreSuggestion is a recommendation to add unnecessary files to .sauceignore in the case that the bundled file is too big.
const SauceIgnoreSuggestion = `We *highly* recommend using .sauceignore file so that saucectl does not create big archives with unnecessary files.

For more information, visit https://docs.saucelabs.com/dev/cli/saucectl/usage/use-cases/#excluding-files-from-the-bundle

or peruse some of our example repositories:
  - https://github.com/saucelabs/saucectl-cypress-example
  - https://github.com/saucelabs/saucectl-playwright-example
  - https://github.com/saucelabs/saucectl-testcafe-example`

// ArchiveFileCountWarning is a warning to the user that their project archive may be unintentionally large.
const ArchiveFileCountWarning = "The project archive is unusually large which can cause delays in your test execution."

// ArchivePathLengthWarning is a warning to the user that their project archive may unintentionally contain long file paths.
const ArchivePathLengthWarning = "The project archive contains paths that exceed the limit of %d characters. This archive may not be usable on Windows."

// WarningLine is a line of starts highlighting the WARNING word.
const WarningLine = "************************************* WARNING *************************************"

// EmptyBuildID indicates it's using empty build ID
const EmptyBuildID = "using empty build ID"

// LogArchiveSizeWarning prints out a warning about the project archive size along with
// suggestions on how to fix it.
func LogArchiveSizeWarning() {
	warning := color.New(color.FgYellow)
	fmt.Printf("\n%s\n\n%s\n\n", warning.Sprint(ArchiveFileCountWarning), SauceIgnoreSuggestion)
}

// LogArchivePathLengthWarning prints out a warning about the project archive
// path length along with suggestions on how to fix it.
func LogArchivePathLengthWarning(limit int) {
	warning := color.New(color.FgYellow)
	fmt.Printf("\n%s\n\n%s\n\n", warning.Sprintf(ArchivePathLengthWarning, limit), SauceIgnoreSuggestion)
}

// LogSauceIgnoreNotExist prints out a formatted and color coded version of SauceIgnoreNotExist.
func LogSauceIgnoreNotExist() {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("\n%s: %s\n\n", red("WARNING"), SauceIgnoreNotExist)
}

// LogGlobalTimeoutShutdown prints out the global timeout shutdown message.
func LogGlobalTimeoutShutdown() {
	color.Red(`┌───────────────────────────────────────────────────┐
│ Global timeout reached. Shutting down saucectl... │
└───────────────────────────────────────────────────┘`)
}

// LogRootDirWarning prints out a warning message regarding the lack of an explicit rootDir configuration.
func LogRootDirWarning() {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("\n%s: %s\n\n", red("WARNING"), "'rootDir' is not defined. Using the current working directory instead "+
		"(equivalent to 'rootDir: .'). Please set 'rootDir' explicitly in your config!")
}

// Error prints out the given message, prefixed with a color coded 'ERROR: ' segment.
func Error(msg string) {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("\n%s: %s\n\n", red("ERROR"), msg)
}

// IgnoredNpmPackagesMsg returns a warning message that framework npm packages are ignored.
func IgnoredNpmPackagesMsg(framework string, installedVersion string, ignoredPackages []string) string {
	return fmt.Sprintf("%s.version (%s) already defined in your config. Ignoring installation of npm packages: %s", framework, installedVersion, strings.Join(ignoredPackages, ", "))
}

// SuiteSplitNoMatch prints the error message due to no files matching pattern found.
func SuiteSplitNoMatch(suiteName, path string, pattern []string) {
	color.Red(fmt.Sprintf("\nNo matching files found for suite '%s'\n", suiteName))
	fmt.Printf("saucectl looked for %s in %s\n\n", strings.Join(pattern, ","), path)
}

func LogUnsupportedPlatform(platform string, supported []string) {
	fmt.Printf("\nThe selected platform %s is not available.\n\n", platform)
	fmt.Println("Available platforms are:")
	var msg string
	for _, p := range supported {
		msg += fmt.Sprintf(" - %s\n", p)
	}
	fmt.Println(msg)
}

func LogConsoleOut(name, logs string) {
	fmt.Printf(`
### CONSOLE OUT %q ###
%s
### CONSOLE END ###
`,
		name, logs)
}

// LogUnsupportedFramework prints out the unsupported framework message.
func LogUnsupportedFramework(frameworkName string) {
	size := len(frameworkName)
	padding := ""
	for i := 0; i < size; i++ {
		padding += "─"
	}
	color.Red(fmt.Sprintf(`
┌─%s───────────────────────────────┐
│ Framework "%s" is not supported  │
└─%s───────────────────────────────┘

`, padding, frameworkName, padding))
}

// EOLNotice generates an EOL message for a given version.
func EOLNotice(name, version string, removalDate time.Time, availableVersions []string) string {
	formattedDate := removalDate.Format("Jan 02, 2006")

	return fmt.Sprintf(
		"%s%s%s%s%s",
		color.RedString(fmt.Sprintf("\n\n%s\n", WarningLine)),
		color.RedString(fmt.Sprintf("\nVersion %s of %s is deprecated and will be removed on %s!\n", version, name, formattedDate)),
		fmt.Sprintf("You should update %s to a more recent version.\n", name),
		color.RedString(fmt.Sprintf("\n%s\n\n", WarningLine)),
		FormatAvailableVersions(name, availableVersions),
	)
}

// RemovalNotice generates a removal message for a given version.
func RemovalNotice(name, version string, availableVersions []string) string {
	return fmt.Sprintf(
		"%s%s%s%s%s",
		color.RedString(fmt.Sprintf("\n\n%s\n", WarningLine)),
		color.RedString(fmt.Sprintf("\nVersion %s of %s is UNSUPPORTED and may be removed at any time!\n", version, name)),
		color.RedString(fmt.Sprintf("You MUST update %s to a more recent version.\n", name)),
		color.RedString(fmt.Sprintf("\n%s\n\n", WarningLine)),
		FormatAvailableVersions(name, availableVersions),
	)
}

// FormatAvailableVersions formats a message listing the available versions.
func FormatAvailableVersions(name string, versions []string) string {
	m := fmt.Sprintf("Available versions of %s are:\n", name)
	for _, v := range versions {
		m += fmt.Sprintf(" - %s\n", v)
	}
	m += "\n"
	return m
}
