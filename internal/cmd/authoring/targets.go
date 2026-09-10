package authoring

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/saucelabs/saucectl/internal/authoring"
)

// ErrNoTargets is returned by commands that require at least one target.
var ErrNoTargets = errors.New("no target specified; use --target or --target-json")

// parseTargets turns the two target flag forms into run targets.
//
// --target takes comma-separated key=value pairs, one flag per target:
//
//	--target browserName=chrome,platformName="Windows 11",browserVersion=latest
//
// Dotted keys nest ("sauce:options.screenResolution=1920x1080"); the values
// "true" and "false" become booleans, everything else stays a string, since
// capability versions like "16" are strings on the wire.
//
// --target-json takes a JSON object, or @path to read one from a file. The
// object is either a capabilities object or a full target
// {"capabilities": {...}, "isRdc": true}.
func parseTargets(kvTargets, jsonTargets []string) ([]authoring.Target, error) {
	var targets []authoring.Target

	for _, kv := range kvTargets {
		t, err := parseKVTarget(kv)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}

	for _, raw := range jsonTargets {
		t, err := parseJSONTarget(raw)
		if err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}

	return targets, nil
}

// parseKVTarget parses one --target value.
func parseKVTarget(kv string) (authoring.Target, error) {
	caps := map[string]any{}
	for _, pair := range splitPairs(kv) {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return authoring.Target{}, fmt.Errorf("invalid --target entry %q: expected key=value", pair)
		}
		setNested(caps, strings.Split(key, "."), coerce(strings.TrimSpace(value)))
	}
	if len(caps) == 0 {
		return authoring.Target{}, fmt.Errorf("invalid --target %q: no capabilities", kv)
	}
	return authoring.Target{Capabilities: caps}, nil
}

// splitPairs splits on commas that are not inside double quotes, so a value
// like "Windows 11" may be quoted to protect spaces and commas. Quotes are
// stripped from the result.
func splitPairs(s string) []string {
	var parts []string
	var b strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ',' && !inQuote:
			parts = append(parts, b.String())
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	parts = append(parts, b.String())
	return parts
}

// coerce turns the literal booleans into bool and leaves everything else a
// string.
func coerce(v string) any {
	switch strings.ToLower(v) {
	case "true":
		return true
	case "false":
		return false
	default:
		return v
	}
}

// setNested assigns value at the dotted path inside m, creating maps as it
// goes. A conflicting scalar on the path is replaced by a map.
func setNested(m map[string]any, path []string, value any) {
	for i, key := range path {
		if i == len(path)-1 {
			m[key] = value
			return
		}
		next, ok := m[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		m = next
	}
}

// parseJSONTarget parses one --target-json value, reading from a file when
// the value starts with @.
func parseJSONTarget(raw string) (authoring.Target, error) {
	if strings.HasPrefix(raw, "@") {
		b, err := os.ReadFile(strings.TrimPrefix(raw, "@"))
		if err != nil {
			return authoring.Target{}, fmt.Errorf("reading --target-json file: %w", err)
		}
		raw = string(b)
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return authoring.Target{}, fmt.Errorf("invalid --target-json: %w", err)
	}
	if len(obj) == 0 {
		return authoring.Target{}, errors.New("invalid --target-json: empty object")
	}

	if capsRaw, ok := obj["capabilities"]; ok {
		caps, ok := capsRaw.(map[string]any)
		if !ok {
			return authoring.Target{}, errors.New("invalid --target-json: capabilities must be an object")
		}
		isRDC, _ := obj["isRdc"].(bool)
		return authoring.Target{Capabilities: caps, IsRDC: isRDC}, nil
	}
	return authoring.Target{Capabilities: obj}, nil
}

// describeTarget renders a target as a short label for tables and logs,
// e.g. "chrome latest / Windows 11" or "Google Pixel 9 Emulator / Android 16".
// The capability lookup itself is shared with the runner's results table via
// authoring.DescribeCapabilities, so both describe a target the same way.
func describeTarget(t authoring.Target) string {
	browser, platform, device := authoring.DescribeCapabilities(t.Capabilities)

	var parts []string
	for _, p := range []string{device, browser, platform} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}
