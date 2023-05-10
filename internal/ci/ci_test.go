package ci

import "fmt"

func ExampleGetCI_github() {
	ci := GetCI(GitHub)
	fmt.Println(ci.Provider.Name)
	// Output: GitHub
}
