package yaml

import (
	"bufio"
	"strings"
)

func SplitLines(s string) (lines []string) {
	var l []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		l = append(l, sc.Text())
	}
	return l
}
