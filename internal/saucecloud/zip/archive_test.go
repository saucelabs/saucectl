package zip

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/v3/fs"
)

func TestExpandDependencies(t *testing.T) {
	dir := fs.NewDir(t, "test-project",
		fs.WithFile("package.json", `{
  "dependencies": {
    "i-am-regular": "0.1.2"
  },
  "devDependencies": {
    "i-am-dev": "0.1.2"
  }
}
`))
	defer dir.Remove()

	deps := []string{"i-am-extra", "package.json"}
	expanded, err := ExpandDependencies(dir.Path(), deps)
	if err != nil {
		t.Error(err)
	}

	expected := []string{"i-am-extra", "i-am-regular", "i-am-dev"}
	if !cmp.Equal(expected, expanded) {
		t.Errorf("unexpected expanded dependencies: %s", cmp.Diff(expected, expanded))
	}
}
