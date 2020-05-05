package progress

import (
	"fmt"
	"time"
	"sync"

	"github.com/briandowns/spinner"
)

type SpinnerSingleton struct {
	Spinner *spinner.Spinner
	Speed time.Duration
	Animation []string
}

var lock = &sync.Mutex{}
var spinnerInstance *SpinnerSingleton

func NewSpinner() (*SpinnerSingleton) {
	/* Using a singleton to stop previous messages
	 * otherwise multiple message can be displayed at the same time 
	 * if we forget to run spinner.Stop()
	*/
	if spinnerInstance == nil {
		lock.Lock()
		defer lock.Unlock()
		if spinnerInstance == nil {
			spinnerInstance = &SpinnerSingleton{}
			spinnerInstance.Speed = 300*time.Millisecond
			spinnerInstance.Animation = []string{"⠋ ", "⠙ ", "⠹ ","⠸ ","⠼ ","⠴ ", "⠦ ", "⠧ ", "⠇ ", "⠏ "}
			spinnerInstance.Spinner = spinner.New(spinnerInstance.Animation, spinnerInstance.Speed)
		}
	}
	return spinnerInstance
}

func Show(text string, args ...interface{}) *spinner.Spinner {
	message := fmt.Sprintf(text, args...)
	spinner := NewSpinner()
	spinner.Spinner.Suffix = message
	spinner.Spinner.Stop()
	spinner.Spinner.Start()
	return spinner.Spinner
}

func Stop() {
	spinner := NewSpinner()
	spinner.Spinner.Stop()
}
