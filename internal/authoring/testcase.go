package authoring

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// TestCase is a saved, reusable test produced from a plain-language
// description. Its identifier is a 24-character hex ObjectId.
type TestCase struct {
	ID    string `json:"id"`
	OrgID string `json:"orgId,omitempty"`
	// TeamID is returned by the detail endpoint but absent from the spec.
	TeamID string `json:"teamId,omitempty"`
	Name   string `json:"name"`
	// Tags are case-sensitive: an organisation may hold both "Login" and
	// "login". Never fold or deduplicate them.
	Tags []string `json:"tags"`
	// TestSuiteID is set when the case belongs to a suite; a case belongs to
	// at most one.
	TestSuiteID string `json:"testSuiteId,omitempty"`
	// Revisions is the full history and is returned even by the list
	// endpoint, which is why a listing weighs ~13 KB per case.
	Revisions            []Revision  `json:"revisions"`
	RunSettings          RunSettings `json:"runSettings"`
	CreationDate         string      `json:"creationDate,omitempty"`
	LastUpdateDate       string      `json:"lastUpdateDate,omitempty"`
	CreatorUserID        string      `json:"creatorUserId,omitempty"`
	CreatorUserName      string      `json:"creatorUserName,omitempty"`
	LastModifierUserID   string      `json:"lastModifierUserId,omitempty"`
	LastModifierUserName string      `json:"lastModifierUserName,omitempty"`
}

// LatestRevision returns the last revision, or false when the case has none. A
// case with no revisions is legitimate and must not be exported.
func (tc TestCase) LatestRevision() (Revision, bool) {
	if len(tc.Revisions) == 0 {
		return Revision{}, false
	}
	return tc.Revisions[len(tc.Revisions)-1], true
}

// Revision is a point-in-time version of a test case: the original intent,
// the steps derived from it, and the reasoning behind them.
type Revision struct {
	ID               string      `json:"id"`
	Intent           string      `json:"intent"`
	Steps            []Step      `json:"steps"`
	DiscoveredIntent string      `json:"discoveredIntent,omitempty"`
	Description      string      `json:"description,omitempty"`
	Reasoning        []Reasoning `json:"reasoning,omitempty"`
}

// Reasoning is one titled paragraph of the agent's reasoning.
type Reasoning struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Step is a single action within a revision.
type Step struct {
	ID   string `json:"id"`
	Tool Tool   `json:"tool"`
	// Result is absent while the outcome is unknown.
	Result *StepResult `json:"result,omitempty"`
	// ScreenshotURL is a fully pre-signed Google Cloud Storage URL (~1 KB,
	// expiring after 24 h), not an identifier. Use ArtifactID to obtain the
	// identifier the storage endpoint accepts.
	ScreenshotURL string `json:"screenshotUrl,omitempty"`
}

// ArtifactID extracts the artifact identifier from ScreenshotURL: the last
// path segment before the query string. Passing it to GET /storage/{id}
// returns the file with Sauce credentials and without expiry. Returns "" when
// there is no screenshot or the URL cannot be parsed.
func (s Step) ArtifactID() string {
	return ArtifactIDFromURL(s.ScreenshotURL)
}

// ArtifactIDFromURL is the URL-to-identifier extraction behind Step.ArtifactID,
// exposed so commands can accept either form from users.
func ArtifactIDFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	id := path.Base(u.Path)
	if id == "." || id == "/" {
		return ""
	}
	return id
}

// StepResult is the outcome of a step.
type StepResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ToolType names the kind of action a step performs.
type ToolType string

// The documented tool types. The set is expected to grow; unknown types are
// rendered by their bare name rather than rejected.
const (
	ToolGoToURL        ToolType = "go_to_url"
	ToolSwitchWindow   ToolType = "switch_window"
	ToolEnterKeySubmit ToolType = "enter_key_submit"
	ToolPause          ToolType = "pause"
	ToolClick          ToolType = "click"
	ToolInputText      ToolType = "input_text"
	ToolScrollDocument ToolType = "scroll_document"
	ToolScrollElement  ToolType = "scroll_element"
	ToolAssert         ToolType = "assert"
	ToolSelect         ToolType = "select"
	ToolInShadowRoot   ToolType = "in_shadow_root"
	ToolFinish         ToolType = "finish"
)

// Tool is the action a step performs. Args is kept as raw JSON rather than
// decoded into twelve typed variants for three reasons: args.matcher is a
// string for switch_window but an object for assert, so one flat struct would
// be incorrect; in_shadow_root nests another Tool recursively; and saucectl
// only ever reads steps — it renders one line and otherwise passes them
// through. Typed variants would silently discard data the day a thirteenth
// type ships. Summary and Reasoning cover the display need and fail soft.
type Tool struct {
	Type ToolType        `json:"type"`
	Args json.RawMessage `json:"args,omitempty"`
}

// Selector locates an element by CSS or XPath.
type Selector struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	// Index picks one match when the selector matches several; nil means the
	// first.
	Index *int `json:"index,omitempty"`
}

// String renders the selector as type=value, e.g. css=#submit, with an [n]
// suffix when an index is set.
func (s Selector) String() string {
	out := s.Type + "=" + s.Value
	if s.Index != nil {
		out = fmt.Sprintf("%s[%d]", out, *s.Index)
	}
	return out
}

// toolArgs is the union of every documented argument across all tool types.
// Decoding into the union is safe because argument names do not collide in
// meaning; the per-type accessors below only read the fields their type
// defines. Matcher stays raw because its shape depends on the type.
type toolArgs struct {
	Reasoning   string          `json:"reasoning"`
	URL         string          `json:"url"`
	Matcher     json.RawMessage `json:"matcher"`
	Time        float64         `json:"time"`
	Selector    *Selector       `json:"selector"`
	XPath       string          `json:"xpath"`
	Text        string          `json:"text"`
	Direction   string          `json:"direction"`
	Not         bool            `json:"not"`
	OptionValue string          `json:"optionValue"`
	Tool        *Tool           `json:"tool"`
}

// assertMatcher is the object form of args.matcher used by assert steps.
type assertMatcher struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}

// decodeArgs decodes Args into the union, reporting false when Args is empty
// or malformed so callers can degrade to the bare tool name.
func (t Tool) decodeArgs() (toolArgs, bool) {
	var a toolArgs
	if len(t.Args) == 0 {
		return a, false
	}
	if err := json.Unmarshal(t.Args, &a); err != nil {
		return a, false
	}
	return a, true
}

// Reasoning returns the agent's stated reason for the step, or "" when the
// arguments are missing or malformed.
func (t Tool) Reasoning() string {
	a, ok := t.decodeArgs()
	if !ok {
		return ""
	}
	return a.Reasoning
}

// Summary renders the step as one readable line, e.g. "click css=#submit" or
// "input_text css=#user ← standard_user". Unknown tool types and malformed
// arguments degrade to the bare type name; they never fail.
func (t Tool) Summary() string {
	name := string(t.Type)
	a, ok := t.decodeArgs()
	if !ok {
		return name
	}

	target := targetOf(a)
	switch t.Type {
	case ToolGoToURL:
		return joinNonEmpty(name, a.URL)
	case ToolSwitchWindow:
		return joinNonEmpty(name, rawString(a.Matcher))
	case ToolPause:
		if a.Time > 0 {
			return fmt.Sprintf("%s %gms", name, a.Time)
		}
		return name
	case ToolClick:
		return joinNonEmpty(name, target)
	case ToolInputText:
		return joinNonEmpty(name, target, "←", a.Text)
	case ToolScrollDocument:
		return joinNonEmpty(name, a.Direction)
	case ToolScrollElement:
		return joinNonEmpty(name, target, a.Direction)
	case ToolAssert:
		not := ""
		if a.Not {
			not = "not"
		}
		return joinNonEmpty(name, not, target, matcherString(a.Matcher))
	case ToolSelect:
		return joinNonEmpty(name, target, "=", a.OptionValue)
	case ToolInShadowRoot:
		if a.Tool != nil {
			return joinNonEmpty(name, target, ">", a.Tool.Summary())
		}
		return joinNonEmpty(name, target)
	case ToolEnterKeySubmit, ToolFinish:
		return name
	default:
		return name
	}
}

// targetOf renders the element a step acts on, preferring the structured
// selector over the deprecated bare xpath.
func targetOf(a toolArgs) string {
	if a.Selector != nil && a.Selector.Value != "" {
		return a.Selector.String()
	}
	if a.XPath != "" {
		return "xpath=" + a.XPath
	}
	return ""
}

// rawString returns a JSON string's value, or the compact JSON for anything
// else, or "" for nothing.
func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// matcherString renders an assert matcher as name(args), e.g.
// toHaveText("2"), degrading to the raw JSON when it is not the documented
// object shape.
func matcherString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m assertMatcher
	if err := json.Unmarshal(raw, &m); err != nil || m.Name == "" {
		return rawString(raw)
	}
	if len(m.Args) == 0 {
		return m.Name
	}
	parts := make([]string, len(m.Args))
	for i, arg := range m.Args {
		b, err := json.Marshal(arg)
		if err != nil {
			parts[i] = fmt.Sprint(arg)
			continue
		}
		parts[i] = string(b)
	}
	return fmt.Sprintf("%s(%s)", m.Name, strings.Join(parts, ", "))
}

// joinNonEmpty joins the non-empty parts with single spaces.
func joinNonEmpty(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " ")
}

// Target is one browser or device to run against: free-form W3C WebDriver
// capabilities passed through untouched, plus whether it is a real device.
// Because capabilities are free-form, the configuration schema needs no
// platform enum and avoids the four-file duplication the other frameworks
// carry. IsRDC is omitted when false so a Target can double as the request
// shape, which declares capabilities only.
type Target struct {
	Capabilities map[string]any `json:"capabilities"`
	IsRDC        bool           `json:"isRdc,omitempty"`
}

// RunSettings are the defaults stored on a test case. A stored case carries
// PrimaryTarget plus RunTargets (plural), whereas an authoring request carries
// a single target — two different shapes that must not be conflated.
type RunSettings struct {
	TestURL string `json:"testUrl,omitempty"`
	// TunnelName may be the empty string, which the service treats as a real
	// tunnel lookup that fails (research Open-3). It is a pointer so that a
	// stored "" survives decoding distinct from an absent field.
	TunnelName    *string  `json:"scTunnelName,omitempty"`
	PrimaryTarget Target   `json:"primaryTarget"`
	RunTargets    []Target `json:"runTargets"`
	LastBuildName string   `json:"lastBuildName,omitempty"`
}

// ListTestCasesOptions filters a test case listing.
type ListTestCasesOptions struct {
	ListOptions
	// Search is a case-insensitive substring match inside words: "demo"
	// matches "Saucedemo - Checkout flow". Exact matching must be done
	// client-side.
	Search    string
	StartDate string
	EndDate   string
	UserID    string
	TeamID    string
	// TestSuiteIDs filters by suite; the literal "null" finds unassigned cases.
	TestSuiteIDs []string
	// Tags matches cases carrying at least one of the tags.
	Tags []string
}
