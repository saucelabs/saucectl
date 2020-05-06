package progress

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
)

var spinnerSpeed = 300 * time.Millisecond
var spinnerInstance *spinner.Spinner = spinner.New(spinner.CharSets[14], spinnerSpeed)

func Show(text string, args ...interface{}) *spinner.Spinner {
	message := " " + fmt.Sprintf(text, args...)
	spinnerInstance.Suffix = message
	spinnerInstance.Stop()
	spinnerInstance.Start()
	return spinnerInstance
}

func Stop() {
	spinnerInstance.Stop()
}
