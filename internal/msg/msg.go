package msg

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// DockerLogo is an eyecatcher message that indicates the user is running tests inside a docker container.
const DockerLogo = `
                                    ##        .
                              ## ## ##       ==
                           ## ## ## ##      ===
                       /""""""""""""""""\___/ ===
                  ~~~ {~~ ~~~~ ~~~ ~~~~ ~~ ~ /  ===- ~~~
                       \______ o          __/
                         \    \        __/
                          \____\______/

  _____   ____   _____ _  ________ _____    __  __  ____  _____  ______ 
 |  __ \ / __ \ / ____| |/ /  ____|  __ \  |  \/  |/ __ \|  __ \|  ____|
 | |  | | |  | | |    | ' /| |__  | |__) | | \  / | |  | | |  | | |__   
 | |  | | |  | | |    |  < |  __| |  _  /  | |\/| | |  | | |  | |  __|  
 | |__| | |__| | |____| . \| |____| | \ \  | |  | | |__| | |__| | |____ 
 |_____/ \____/ \_____|_|\_\______|_|  \_\ |_|  |_|\____/|_____/|______|
`

// SauceLogo is an eyecatcher message that indicates the user is running tests in the Sauce Labs cloud.
const SauceLogo = `
                                        (.                          
                                       #.                           
                                       #.                           
                           .####################                    
                         #####////////*******/######                
                       .##///////*****************###/              
                      ,###////*********************###              
                      ####//***********************####             
                       ###/************************###              
                        ######********************###. ##           
                           (########################  ##     ##     
                                   ,######(#*         ##*   (##     
                               /############*          #####        
                           (########(  #########(    ###            
                         .#######,    */  ############              
                      ,##########  %#### , ########*                
                    *### .#######/  ##  / ########                  
                   ###   .###########//###########                  
               ######     ########################                  
             (#(    *#(     #######.    (#######                    
                    ##,    /########    ########                    
                           *########    ########                    

   _____        _    _  _____ ______    _____ _      ____  _    _ _____  
  / ____|  /\  | |  | |/ ____|  ____|  / ____| |    / __ \| |  | |  __ \ 
 | (___   /  \ | |  | | |    | |__    | |    | |   | |  | | |  | | |  | |
  \___ \ / /\ \| |  | | |    |  __|   | |    | |   | |  | | |  | | |  | |
  ____) / ____ \ |__| | |____| |____  | |____| |___| |__| | |__| | |__| |
 |_____/_/    \_\____/ \_____|______|  \_____|______\____/ \____/|_____/
`

// SignupMessage explains how to obtain a Sauce Labs account and where to find the access key.
const SignupMessage = `Don't have an account? Signup here:
https://bit.ly/saucectl-signup

Already have an account? Get your username and access key here:
https://app.saucelabs.com/user-settings`

// SauceIgnoreNotExist is a recommendation to create a .sauceignore file in the case that it is missing.
const SauceIgnoreNotExist = `The .sauceignore file does not exist. We *highly* recommend creating one so that saucectl does not
create archives with unnecessary files. You are very likely to experience longer startup times.

For more information, visit https://docs.saucelabs.com/testrunner-toolkit/configuration/bundling/index.html#exclude-files-from-the-bundle

or peruse some of our example repositories:
  - https://github.com/saucelabs/saucectl-cypress-example
  - https://github.com/saucelabs/saucectl-playwright-example
  - https://github.com/saucelabs/saucectl-puppeteer-example
  - https://github.com/saucelabs/saucectl-testcafe-example`

// UploadingTimeoutSuggestion is a recommendation to add unnecessary files to .sauceignore in the case that the bundled file is too big.
const UploadingTimeoutSuggestion = `We *highly* recommend using .sauceignore file so that saucectl does not
create big archives with unnecessary files.

For more information, visit https://docs.saucelabs.com/testrunner-toolkit/configuration/bundling/index.html#exclude-files-from-the-bundle

or peruse some of our example repositories:
  - https://github.com/saucelabs/saucectl-cypress-example
  - https://github.com/saucelabs/saucectl-playwright-example
  - https://github.com/saucelabs/saucectl-puppeteer-example
  - https://github.com/saucelabs/saucectl-testcafe-example`

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

// LogUploadTimeout prints out a timeout warning.
func LogUploadTimeout() {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("\n%s: %s\n\n", red("TIMEOUT"), UploadingTimeout)
}

// LogUploadTimeoutSuggestion prints out adding unnecessary files to .sauceignore
func LogUploadTimeoutSuggestion() {
	fmt.Printf("%s\n\n", UploadingTimeoutSuggestion)
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

// PathTooLongForArchive prints the error message due to some filepath being too long.
func PathTooLongForArchive(path string) {
	color.Red("\nSome of your filepaths are too long (200 char limit) !\n\n")
	fmt.Printf("Example: %s\n\n", path)
	fmt.Printf("If you didn't mean to include those files, exclude them via the .sauceignore file.\nIf you need to include those files, then you have to shorten the filepath, for example, by renaming files, folders or avoid nesting files too deeply.\n\n")
}

// SuiteSplitNoMatch prints the error message due to no files matching pattern found.
func SuiteSplitNoMatch(suiteName, path string, pattern []string) {
	color.Red(fmt.Sprintf("\nNo matching files found for suite '%s'\n", suiteName))
	fmt.Printf("saucectl looked for the %s in %s\n\n", strings.Join(pattern, ","), path)
}
