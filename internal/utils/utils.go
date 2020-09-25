package utils

import (
	"bufio"
	"strings"
)

// SplitLines convert string into []string based on line-breaks
func SplitLines(s string) ([]string) {
	var l []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		l = append(l, sc.Text())
	}
	return l
}
