package msg

import (
	"fmt"
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

// SauceLoog is an eyecatcher message that indicates the user is running tests in the Sauce Labs cloud.
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
