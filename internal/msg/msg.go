package msg

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	"strings"
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
 |_____/ \____/ \_____|_|\_\______|_|  \_\ |_|  |_|\____/|_____/|______|`

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
 |_____/_/    \_\____/ \_____|______|  \_____|______\____/ \____/|_____/`

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

// LogSauceIgnoreNotExist prints out a formatted and color coded version of SauceIgnoreNotExist.
func LogSauceIgnoreNotExist() {
	red := color.New(color.FgRed).SprintFunc()
	fmt.Printf("\n%s: %s\n\n", red("WARNING"), SauceIgnoreNotExist)
}

// LogTestSuccess prints out a test success summary statement.
func LogTestSuccess() {
	log.Info().Msg("┌───────────────────────┐")
	log.Info().Msg(" All suites have passed! ")
	log.Info().Msg("└───────────────────────┘")
}

// LogTestFailure prints out a test failure summary statement.
func LogTestFailure(errors, total int) {
	relative := float64(errors) / float64(total) * 100
	msg := fmt.Sprintf(" %d of %d suites have failed (%.0f%%) ", errors, total, relative)
	dashes := strings.Repeat("─", len(msg)-2)
	log.Error().Msgf("┌%s┐", dashes)
	log.Error().Msg(msg)
	log.Error().Msgf("└%s┘", dashes)
}
