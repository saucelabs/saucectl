package framework

import "testing"

func TestGitReleaseSegments(t *testing.T) {
	type args struct {
		m *Metadata
	}
	tests := []struct {
		name     string
		args     args
		wantOrg  string
		wantRepo string
		wantTag  string
		wantErr  bool
	}{
		{
			name: "the regular usecase",
			args: args{
				&Metadata{
					GitRelease: "sauce/this-is-spicy:v1",
				},
			},
			wantOrg:  "sauce",
			wantRepo: "this-is-spicy",
			wantTag:  "v1",
			wantErr:  false,
		},
		{
			name: "malformed",
			args: args{
				&Metadata{
					GitRelease: "totally random string",
				},
			},
			wantErr: true,
		},
		{
			name: "empty",
			args: args{
				&Metadata{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOrg, gotRepo, gotTag, err := GitReleaseSegments(tt.args.m)
			if (err != nil) != tt.wantErr {
				t.Errorf("GitReleaseSegments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOrg != tt.wantOrg {
				t.Errorf("GitReleaseSegments() gotOrg = %v, want %v", gotOrg, tt.wantOrg)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("GitReleaseSegments() gotRepo = %v, want %v", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Errorf("GitReleaseSegments() gotTag = %v, want %v", gotTag, tt.wantTag)
			}
		})
	}
}
