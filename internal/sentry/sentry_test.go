package sentry

import "testing"

func Test_attachmentURLFromDSN(t *testing.T) {
	type args struct {
		dsn     string
		eventID string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "happy path",
			args: args{
				dsn:     "https://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "https://o448931.ingest.sentry.io/api/1234567/events/123/attachments/?sentry_key=a5678abc12c590d32fg9f49643072bcf",
			wantErr: false,
		},
		{
			name: "no schema",
			args: args{
				dsn:     "://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "no user",
			args: args{
				dsn:     "https://o448931.ingest.sentry.io/1234567",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "no project ID",
			args: args{
				dsn:     "https://a5678abc12c590d32fg9f49643072bcf@o448931.ingest.sentry.io",
				eventID: "123",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attachmentURLFromDSN(tt.args.dsn, tt.args.eventID)
			if (err != nil) != tt.wantErr {
				t.Errorf("attachmentURLFromDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("attachmentURLFromDSN() got = %v, want %v", got, tt.want)
			}
		})
	}
}
