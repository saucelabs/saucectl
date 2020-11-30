package jsonio

import (
	"bytes"
	"encoding/json"
	"gotest.tools/v3/fs"
	"io/ioutil"
	"reflect"
	"testing"
)

func TestWriteFile(t *testing.T) {
	tmpDir := fs.NewDir(t, "jsonio-filedump")
	defer tmpDir.Remove()

	type TestDTO struct {
		S string
	}

	type args struct {
		name string
		v    interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "write it out",
			args: args{
				name: tmpDir.Join("hello.json"),
				v:    TestDTO{S: "world"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := WriteFile(tt.args.name, tt.args.v); (err != nil) != tt.wantErr {
				t.Errorf("WriteFile() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Read back what was written out and compare against original object
				var dto TestDTO
				fromFile(t, tt.args.name, &dto)
				if !reflect.DeepEqual(dto, tt.args.v) {
					t.Errorf("WriteFile() got = %+v, want %+v", dto, tt.args.v)
				}
			}
		})
	}
}

func fromFile(t *testing.T, filename string, v interface{}) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Error(err)
	}
	r := bytes.NewReader(b)
	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Error(err)
	}
}
