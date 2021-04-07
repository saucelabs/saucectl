package yaml

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFile(t *testing.T) {
	type sample struct {
		Msg string `yaml:"msg"`
	}
	
	type args struct {
		name string
		v    interface{}
		mode os.FileMode
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				name: filepath.Join(os.TempDir(), "saucy.yml"),
				v:    sample{Msg: "hello world!"},
				mode: 0777,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := WriteFile(tt.args.name, tt.args.v, tt.args.mode); (err != nil) != tt.wantErr {
				t.Errorf("WriteFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
