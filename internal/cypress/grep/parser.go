package grep

import "strings"

type Predicate interface {
	Match(input string) bool
}

type Query struct {
	Search string
	Invert bool
}

type Any struct {
	Predicates []Predicate
}

type Every struct {
	Predicates []Predicate
}

func (q Query) Match(input string) bool {
	if q.Invert {
		return !strings.Contains(input, q.Search)
	}
	return strings.Contains(input, q.Search)
}

func (a Any) Match(input string) bool {
	result := false
	for _, p := range a.Predicates {
		result = result || p.Match(input)
		if result {
			break
		}
	}
	return result
}

func (a *Any) Add(p Predicate) {
	a.Predicates = append(a.Predicates, p)
}

func (e Every) Match(input string) bool {
	result := true
	for _, p := range e.Predicates {
		result = result && p.Match(input)
		if !result {
			break
		}
	}

	return result
}

func (e *Every) Add(p Predicate) {
	e.Predicates = append(e.Predicates, p)
}

func Parse(expression string) Predicate {
	strs := strings.Split(expression, ";")
	strs = trimAll(strs)

	substringMatches := Any{}
	invertedMatches := Every{}
	for _, s := range strs {
		if strings.HasPrefix(s, "-") {
			invertedMatches.Add(Query{
				Search: s[1:],
				Invert: true,
			})
			continue
		}
		substringMatches.Add(Query{
			Search: s,
		})
	}

	parsed := Every{}
	parsed.Add(invertedMatches)
	if len(substringMatches.Predicates) > 0 {
		parsed.Add(substringMatches)
	}
	return parsed
}

func trimAll(strs []string) []string {
	var all []string
	for _, s := range strs {
		all = append(all, strings.TrimSpace(s))
	}
	return all
}
