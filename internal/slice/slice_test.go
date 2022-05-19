package slice

import (
	"fmt"
)

func ExampleJoin_strings() {
	fmt.Println(Join([]string{"a", "b", "c"}, ","))
	// Output: a,b,c
}

func ExampleJoin_ints() {
	fmt.Println(Join([]int{1, 2, 3}, ","))
	// Output: 1,2,3
}

func ExampleJoin_interfaces() {
	fmt.Println(Join([]interface{}{"a", "b", "c"}, ","))
	// Output: a,b,c
}

func ExampleJoin_string() {
	fmt.Println(Join("hello", ","))
	// Output: hello
}

func ExampleJoin_struct() {
	type Person struct {
		Name string
	}
	fmt.Println(Join(Person{Name: "Someone Else"}, ","))
	// Output: {Someone Else}
}
