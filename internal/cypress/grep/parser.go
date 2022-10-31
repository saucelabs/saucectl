package grep

import (
	"regexp"
	"strings"
)

var reDelimiter = regexp.MustCompile(`[ ,]`)

// Expression is an interface that represents a logical statement.
type Expression interface {
	// Eval evaluates the Expression against the input string.
	Eval(input string) bool
}

// Partial represents an expression that wraps a string to be matched
// and evaluates to true if that string is a partial substring of the input.
type Partial struct {
	Query  string
	Invert bool
}

// Exact represents an expression that wraps a string to be matched
// and evalutates to true if that string is an exact substring of the input.
type Exact struct {
	Query  string
	Invert bool
}

// Any represents a composite expression that evaluates to true if any
// child expression evalutes to true.
type Any struct {
	Expressions []Expression
}

// All represents a compositite expression thave evaluates to true if all
// child expressions evaluate to true.
type All struct {
	Expressions []Expression
}

func (q Partial) Eval(input string) bool {
	if q.Invert {
		return !strings.Contains(input, q.Query)
	}
	return strings.Contains(input, q.Query)
}

func (q Exact) Eval(input string) bool {
	words := strings.Split(input, " ")
	contains := false

	for _, w := range words {
		if w == q.Query {
			contains = true
			break
		}
	}

	if q.Invert {
		return !contains
	}
	return contains
}

func (a Any) Eval(input string) bool {
	for _, p := range a.Expressions {
		if p.Eval(input) {
			return true
		}
	}
	return false
}

func (a *Any) add(p Expression) {
	a.Expressions = append(a.Expressions, p)
}

func (e All) Eval(input string) bool {
	for _, p := range e.Expressions {
		if !p.Eval(input) {
			return false
		}
	}
	return true
}

func (e *All) add(p Expression) {
	e.Expressions = append(e.Expressions, p)
}

// ParseGrepExp parses a cypress-grep expression and returns an Expression.
//
// The returned Expression can be used to evaluate whether a given string
// can match the expression.
func ParseGrepExp(expr string) Expression {
	strs := strings.Split(expr, ";")
	strs = normalize(strs)

	substringMatches := Any{}
	invertedMatches := All{}
	for _, s := range strs {
		if strings.HasPrefix(s, "-") {
			invertedMatches.add(Partial{
				Query:  s[1:],
				Invert: true,
			})
			continue
		}
		substringMatches.add(Partial{
			Query: s,
		})
	}

	parsed := All{}
	parsed.add(invertedMatches)
	if len(substringMatches.Expressions) > 0 {
		parsed.add(substringMatches)
	}
	return parsed
}

// ParseGrepTagsExp parses a cypress-grep tag expression.
//
// The returned Expression can be used to evaluate whether a given string
// can match the expression.
func ParseGrepTagsExp(expr string) Expression {
	exprs := reDelimiter.Split(expr, -1)
	exprs = normalize(exprs)

	var parsed Any
	var not []Expression

	// Find any global inverted expressions first
	for _, e := range exprs {
		if strings.HasPrefix(e, "--") {
			not = append(not, Exact{
				Query:  e[2:],
				Invert: true,
			})
		}
	}

	for _, e := range exprs {
		if strings.HasPrefix(e, "--") {
			continue
		}

		matcher := All{}
		patterns := strings.Split(e, "+")
		patterns = normalize(patterns)
		for _, p := range patterns {
			invert := false
			if strings.HasPrefix(p, "-") {
				invert = true
				p = p[1:]
			}

			matcher.add(Exact{
				Query:  p,
				Invert: invert,
			})
		}

		// Add any globally inverted expressions found above
		for _, n := range not {
			matcher.add(n)
		}

		parsed.add(matcher)
	}

	return parsed
}

// normalize trims leading and trailing whitespace from a slice of strings
// and filters out any strings that contain only whitespace.
func normalize(strs []string) []string {
	var all []string
	for _, s := range strs {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		all = append(all, strings.TrimSpace(s))
	}
	return all
}
