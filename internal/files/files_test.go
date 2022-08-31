package files

import "testing"
import "gotest.tools/v3/fs"

func TestNewSHA256(t *testing.T) {
	dir := fs.NewDir(t, "checksums", fs.WithFile("hello.txt", "world!"))
	defer dir.Remove()

	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "compute checksum",
			args: args{
				dir.Join("hello.txt"),
			},
			want:    "711e9609339e92b03ddc0a211827dba421f38f9ed8b9d806e1ffdd8c15ffa03d",
			wantErr: false,
		},
		{
			name: "file does not exist",
			args: args{
				dir.Join("rude.txt"),
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSHA256(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSHA256() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NewSHA256() got = %v, want %v", got, tt.want)
			}
		})
	}
}
