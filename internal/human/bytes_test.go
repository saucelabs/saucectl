package human

import "fmt"

func ExampleBytes_gigaByte() {
	fmt.Println(Bytes(3000000000))
	// Output: 3 GB
}

func ExampleBytes_megaByte() {
	fmt.Println(Bytes(82854982))
	// Output: 83 MB
}

func ExampleBytes_kiloByte() {
	fmt.Println(Bytes(828549))
	// Output: 829 kB
}
