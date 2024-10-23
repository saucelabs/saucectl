package grep

import (
	"regexp"
	"strings"
)

// Functions to parse cypress-grep expressions.
// cypress-grep expressions can include simple logical operations (e.g.
// NOT, AND, OR). These expressions are parsed into Expressions that are
// logical predicates that evaluate to true or false depending if the expression matches
// a given input string.

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
// child expression evaluates to true.
type Any struct {
	Expressions []Expression
}

// All represents a composite expression that evaluates to true if all
// child expressions evaluate to true.
type All struct {
	Expressions []Expression
}

// Literal represents an expression that always evaluates to a literal truth value.
type Literal struct {
	value bool
}

func (l Literal) Eval(string) bool {
	return l.value
}

func (p Partial) Eval(input string) bool {
	contains := strings.Contains(input, p.Query)
	if p.Invert {
		return !contains
	}
	return contains
}

func (e Exact) Eval(input string) bool {
	words := strings.Split(input, " ")
	contains := false

	for _, w := range words {
		if w == e.Query {
			contains = true
			break
		}
	}

	if e.Invert {
		return !contains
	}
	return contains
}

func (a *Any) Eval(input string) bool {
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

func (a *All) Eval(input string) bool {
	for _, p := range a.Expressions {
		if !p.Eval(input) {
			return false
		}
	}
	return true
}

func (a *All) add(p Expression) {
	a.Expressions = append(a.Expressions, p)
}

// ParseGrepTitleExp parses a cypress-grep expression and returns an Expression.
//
// The returned Expression can be used to evaluate whether a given string
// can match the expression.
func ParseGrepTitleExp(expr string) Expression {
	// Treat an empty expression as if grepping were disabled
	if expr == "" {
		return Literal{
			value: true,
		}
	}
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
	parsed.add(&invertedMatches)
	if len(substringMatches.Expressions) > 0 {
		parsed.add(&substringMatches)
	}
	return &parsed
}

// ParseGrepTagsExp parses a cypress-grep tag expression.
//
// The returned Expression can be used to evaluate whether a given string
// can match the expression.
func ParseGrepTagsExp(expr string) Expression {
	// Treat an empty expression as if grepping were disabled
	if expr == "" {
		return Literal{
			value: true,
		}
	}
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

		parsed.add(&matcher)
	}

	return &parsed
}

// normalize trims leading and trailing whitespace from a slice of strings
//  and filters out any strings that contain only whitespace.
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
