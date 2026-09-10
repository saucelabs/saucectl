package authoring

import (
	"encoding/json"
	"testing"
)

func TestTool_Summary(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
		want string
	}{
		{
			name: "go_to_url",
			tool: Tool{Type: ToolGoToURL, Args: json.RawMessage(`{"reasoning":"open","url":"https://saucedemo.com"}`)},
			want: "go_to_url https://saucedemo.com",
		},
		{
			name: "switch_window with string matcher",
			tool: Tool{Type: ToolSwitchWindow, Args: json.RawMessage(`{"reasoning":"r","matcher":"Checkout"}`)},
			want: "switch_window Checkout",
		},
		{
			name: "switch_window with unexpected object matcher degrades to json",
			tool: Tool{Type: ToolSwitchWindow, Args: json.RawMessage(`{"matcher":{"title":"x"}}`)},
			want: `switch_window {"title":"x"}`,
		},
		{
			name: "enter_key_submit",
			tool: Tool{Type: ToolEnterKeySubmit, Args: json.RawMessage(`{"reasoning":"submit"}`)},
			want: "enter_key_submit",
		},
		{
			name: "pause in milliseconds",
			tool: Tool{Type: ToolPause, Args: json.RawMessage(`{"reasoning":"wait","time":1500}`)},
			want: "pause 1500ms",
		},
		{
			name: "pause without time",
			tool: Tool{Type: ToolPause, Args: json.RawMessage(`{"reasoning":"wait"}`)},
			want: "pause",
		},
		{
			name: "click with css selector",
			tool: Tool{Type: ToolClick, Args: json.RawMessage(`{"reasoning":"r","selector":{"type":"css","value":"#submit"}}`)},
			want: "click css=#submit",
		},
		{
			name: "click with indexed selector",
			tool: Tool{Type: ToolClick, Args: json.RawMessage(`{"selector":{"type":"css","value":".item","index":2}}`)},
			want: "click css=.item[2]",
		},
		{
			name: "click falls back to deprecated xpath",
			tool: Tool{Type: ToolClick, Args: json.RawMessage(`{"reasoning":"r","xpath":"//a[1]"}`)},
			want: "click xpath=//a[1]",
		},
		{
			name: "input_text",
			tool: Tool{Type: ToolInputText, Args: json.RawMessage(`{"selector":{"type":"css","value":"#user"},"text":"standard_user"}`)},
			want: "input_text css=#user ← standard_user",
		},
		{
			name: "scroll_document",
			tool: Tool{Type: ToolScrollDocument, Args: json.RawMessage(`{"direction":"down"}`)},
			want: "scroll_document down",
		},
		{
			name: "scroll_element",
			tool: Tool{Type: ToolScrollElement, Args: json.RawMessage(`{"selector":{"type":"xpath","value":"//ul"},"direction":"up"}`)},
			want: "scroll_element xpath=//ul up",
		},
		{
			name: "assert with matcher args",
			tool: Tool{Type: ToolAssert, Args: json.RawMessage(`{"selector":{"type":"css","value":".badge"},"matcher":{"name":"toHaveText","args":["2"]}}`)},
			want: `assert css=.badge toHaveText("2")`,
		},
		{
			name: "assert negated without matcher args",
			tool: Tool{Type: ToolAssert, Args: json.RawMessage(`{"not":true,"selector":{"type":"css","value":".error"},"matcher":{"name":"toBeVisible"}}`)},
			want: "assert not css=.error toBeVisible",
		},
		{
			name: "assert with multiple matcher args of mixed types",
			tool: Tool{Type: ToolAssert, Args: json.RawMessage(`{"selector":{"type":"css","value":"#n"},"matcher":{"name":"toHaveCount","args":[3,true]}}`)},
			want: "assert css=#n toHaveCount(3, true)",
		},
		{
			name: "select",
			tool: Tool{Type: ToolSelect, Args: json.RawMessage(`{"selector":{"type":"css","value":"#country"},"optionValue":"eu"}`)},
			want: "select css=#country = eu",
		},
		{
			name: "in_shadow_root recurses into the nested tool",
			tool: Tool{Type: ToolInShadowRoot, Args: json.RawMessage(`{"selector":{"type":"css","value":"my-host"},"tool":{"type":"click","args":{"selector":{"type":"css","value":"#inner"}}}}`)},
			want: "in_shadow_root css=my-host > click css=#inner",
		},
		{
			name: "in_shadow_root without nested tool",
			tool: Tool{Type: ToolInShadowRoot, Args: json.RawMessage(`{"selector":{"type":"css","value":"my-host"}}`)},
			want: "in_shadow_root css=my-host",
		},
		{
			name: "finish",
			tool: Tool{Type: ToolFinish, Args: json.RawMessage(`{"reasoning":"done"}`)},
			want: "finish",
		},
		{
			name: "unknown type degrades to its name",
			tool: Tool{Type: "teleport", Args: json.RawMessage(`{"where":"home"}`)},
			want: "teleport",
		},
		{
			name: "malformed args degrade to the type name",
			tool: Tool{Type: ToolClick, Args: json.RawMessage(`{not json`)},
			want: "click",
		},
		{
			name: "nil args degrade to the type name",
			tool: Tool{Type: ToolInputText},
			want: "input_text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tool.Summary(); got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTool_Reasoning(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
		want string
	}{
		{"present", Tool{Type: ToolFinish, Args: json.RawMessage(`{"reasoning":"all items added"}`)}, "all items added"},
		{"absent", Tool{Type: ToolFinish, Args: json.RawMessage(`{}`)}, ""},
		{"malformed", Tool{Type: ToolFinish, Args: json.RawMessage(`{`)}, ""},
		{"nil", Tool{Type: ToolFinish}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tool.Reasoning(); got != tt.want {
				t.Errorf("Reasoning() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTool_RoundTripKeepsUnknownArgs(t *testing.T) {
	// Args are raw so a thirteenth tool type, or a new argument on an existing
	// one, survives decode and re-encode unchanged.
	in := `{"type":"click","args":{"reasoning":"r","selector":{"type":"css","value":"#a"},"newField":{"nested":true}}}`
	var tool Tool
	if err := json.Unmarshal([]byte(in), &tool); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(tool)
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal([]byte(in), &a)
	_ = json.Unmarshal(out, &b)
	if a["args"].(map[string]any)["newField"] == nil || b["args"].(map[string]any)["newField"] == nil {
		t.Errorf("unknown argument was lost in round trip: %s", out)
	}
}

func TestArtifactIDFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "pre-signed storage url",
			url:  "https://storage.googleapis.com/bucket/org/steps/3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b?X-Goog-Algorithm=GOOG4-RSA-SHA256&X-Goog-Expires=86400&X-Goog-Signature=abc",
			want: "3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b",
		},
		{name: "bare identifier passes through", url: "3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b", want: "3f2a9c1e4b5d6e7f8a9b0c1d2e3f4a5b"},
		{name: "empty", url: "", want: ""},
		{name: "unparseable", url: "://nope", want: ""},
		{name: "host only", url: "https://example.com/", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ArtifactIDFromURL(tt.url); got != tt.want {
				t.Errorf("ArtifactIDFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestTestCase_LatestRevision(t *testing.T) {
	var empty TestCase
	if _, ok := empty.LatestRevision(); ok {
		t.Error("expected no revision on an empty test case")
	}
	tc := TestCase{Revisions: []Revision{{ID: "old"}, {ID: "new"}}}
	rev, ok := tc.LatestRevision()
	if !ok || rev.ID != "new" {
		t.Errorf("LatestRevision() = %v, %v; want the last revision", rev, ok)
	}
}

func TestRunSettings_EmptyTunnelNameSurvivesDecoding(t *testing.T) {
	// The whole reason TunnelName is a pointer: a stored "" must remain
	// distinguishable from an absent field (research R-008).
	var withEmpty, without RunSettings
	if err := json.Unmarshal([]byte(`{"scTunnelName":"","primaryTarget":{"capabilities":{}},"runTargets":[]}`), &withEmpty); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"primaryTarget":{"capabilities":{}},"runTargets":[]}`), &without); err != nil {
		t.Fatal(err)
	}
	if withEmpty.TunnelName == nil || *withEmpty.TunnelName != "" {
		t.Errorf("expected an empty-string tunnel name, got %v", withEmpty.TunnelName)
	}
	if without.TunnelName != nil {
		t.Errorf("expected nil tunnel name, got %q", *without.TunnelName)
	}
}
