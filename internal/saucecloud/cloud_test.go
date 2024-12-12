package saucecloud

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_arrayContains(t *testing.T) {
	type args struct {
		list []string
		want string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty set",
			args: args{
				list: []string{},
				want: "value",
			},
			want: false,
		},
		{
			name: "Complete set - false",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "value",
			},
			want: false,
		},
		{
			name: "Found",
			args: args{
				list: []string{"val1", "val2", "val3"},
				want: "val1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, arrayContains(tt.args.list, tt.args.want), "arrayContains(%v, %v)", tt.args.list, tt.args.want)
		})
	}
}

type MockMatcher struct {
	ignoreNodeModules bool
}

func (m *MockMatcher) Match(path []string, _ bool) bool {
	return m.ignoreNodeModules && strings.Contains(filepath.Join(path...), "node_modules")
}

func TestCloudRunner_needsNodeModules(t *testing.T) {
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "node_modules")
	dependencies := []string{"chalk", "lodash"}

	createNodeModules := func() {
		if err := os.Mkdir(modDir, 0755); err != nil {
			t.Fatalf("failed to create node_modules directory: %v", err)
		}
	}

	tests := []struct {
		name          string
		setup         func()
		ignoreModules bool
		dependencies  []string
		want          bool
		expectErr     bool
	}{
		{
			name:          "No dependencies, no node_modules",
			setup:         func() {},
			ignoreModules: false,
			dependencies:  []string{},
			want:          false,
			expectErr:     false,
		},
		{
			name:          "Dependencies defined, no node_modules",
			setup:         func() {},
			ignoreModules: false,
			dependencies:  dependencies,
			want:          false,
			expectErr:     true,
		},
		{
			name:          "Dependencies defined, node_modules present",
			setup:         createNodeModules,
			ignoreModules: false,
			dependencies:  dependencies,
			want:          true,
			expectErr:     false,
		},
		{
			name:          "Dependencies defined, node_modules ignored",
			setup:         createNodeModules,
			ignoreModules: true,
			dependencies:  dependencies,
			want:          false,
			expectErr:     true,
		},
		{
			name:          "No dependencies, node_modules ignored",
			setup:         createNodeModules,
			ignoreModules: true,
			dependencies:  []string{},
			want:          false,
			expectErr:     false,
		},
		{
			name:          "No dependencies, node_modules present",
			setup:         createNodeModules,
			ignoreModules: false,
			dependencies:  []string{},
			want:          true,
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			t.Cleanup(func() {
				if err := os.RemoveAll(modDir); err != nil {
					t.Fatalf("failed to clean up node_modules directory: %v", err)
				}
			})

			matcher := &MockMatcher{ignoreNodeModules: tt.ignoreModules}
			got, err := needsNodeModules(tempDir, matcher, tt.dependencies)

			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error: %v, got error: %v", tt.expectErr, err)
			}

			if got != tt.want {
				t.Errorf("expected result: %v, got result: %v", tt.want, got)
			}
		})
	}
}
