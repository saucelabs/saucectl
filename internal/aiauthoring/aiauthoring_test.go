package aiauthoring

import (
	"encoding/json"
	"testing"
)

func TestRunRequest_MarshalJSON(t *testing.T) {
	testCases := []struct {
		name string
		req  RunRequest
		want string
	}{
		{
			name: "tunnel omitted when unset",
			req:  RunRequest{BuildName: "b"},
			want: `{"buildName":"b"}`,
		},
		{
			name: "tunnel name set",
			req:  RunRequest{SCTunnelName: "my-tunnel"},
			want: `{"scTunnelName":"my-tunnel"}`,
		},
		{
			name: "explicit null tunnel when cleared",
			req:  RunRequest{ClearTunnel: true},
			want: `{"scTunnelName":null}`,
		},
		{
			name: "tunnel name wins over clearing",
			req:  RunRequest{SCTunnelName: "my-tunnel", ClearTunnel: true},
			want: `{"scTunnelName":"my-tunnel"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}
