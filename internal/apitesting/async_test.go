package apitesting

import (
	"context"
	"github.com/saucelabs/saucectl/internal/config"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClient_RunAllAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
			},
			want: AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/_run-all":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &Client{
		HTTPClient: ts.Client(),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunAllAsync(tt.args.ctx, tt.args.hookID, tt.args.buildID, tt.args.tunnel)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_RunEphemeralAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tunnel  config.Tunnel
		taskID  string
		test    TestRequest
	}
	tests := []struct {
		name          string
		args          args
		assertRequest func(t *testing.T, r *http.Request)
		reply         []byte
		want          AsyncResponse
		wantErr       bool
	}{
		{
			name: "Complete Trigger",
			args: args{
				ctx:     context.Background(),
				hookID:  "dummyHookId",
				taskID:  "generatedUuid",
				buildID: "generatedBuildId",
				tunnel:  config.Tunnel{Name: "tunnelId"},
				test:    TestRequest{},
			},
			assertRequest: func(t *testing.T, r *http.Request) {
				assert.Equal(t, "/api-testing/rest/v4/dummyHookId/tests/_exec?buildId=generatedBuildId&tunnelId=tunnelId", r.RequestURI)
			},
			reply: []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`),
			want: AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusNotImplemented)
					return
				}
				switch r.URL.Path {
				case "/api-testing/rest/v4/dummyHookId/tests/_exec":
					tt.assertRequest(t, r)
					completeStatusResp := tt.reply
					_, _ = w.Write(completeStatusResp)
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer ts.Close()
			c := &Client{
				HTTPClient: ts.Client(),
				URL:        ts.URL,
				Username:   "dummyUser",
				AccessKey:  "dummyAccesKey",
			}

			got, err := c.RunEphemeralAsync(tt.args.ctx, tt.args.hookID, tt.args.buildID, tt.args.tunnel, tt.args.taskID, tt.args.test)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_RunTestAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		testID  string
		hookID  string
		buildID string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
				testID: "c20ad4d76fe97759aa27a0c9",
			},
			want: AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/c20ad4d76fe97759aa27a0c9/_run":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &Client{
		HTTPClient: ts.Client(),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunTestAsync(tt.args.ctx, tt.args.hookID, tt.args.testID, tt.args.buildID, tt.args.tunnel)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_RunTagAsync(t *testing.T) {
	type args struct {
		ctx     context.Context
		hookID  string
		buildID string
		tagID   string
		tunnel  config.Tunnel
	}
	tests := []struct {
		name    string
		args    args
		want    AsyncResponse
		wantErr bool
	}{
		{
			name: "Basic trigger",
			args: args{
				ctx:    context.Background(),
				hookID: "dummyHookId",
				tagID:  "dummyTag",
			},
			want: AsyncResponse{
				ContextIDs: []string{"221270ac-0229-49d1-9025-251a10e9133d"},
				EventIDs:   []string{"c4ca4238a0b923820dcc509a"},
				TaskID:     "6ddf80b7-9753-4802-992b-d42948cdb99f",
				TestIDs:    []string{"c20ad4d76fe97759aa27a0c9"},
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		switch r.URL.Path {
		case "/api-testing/rest/v4/dummyHookId/tests/_tag/dummyTag/_run":
			completeStatusResp := []byte(`{"contextIds":["221270ac-0229-49d1-9025-251a10e9133d"],"eventIds":["c4ca4238a0b923820dcc509a"],"taskId":"6ddf80b7-9753-4802-992b-d42948cdb99f","testIds":["c20ad4d76fe97759aa27a0c9"]}`)
			_, err = w.Write(completeStatusResp)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}

		if err != nil {
			t.Errorf("failed to respond: %v", err)
		}
	}))
	defer ts.Close()
	c := &Client{
		HTTPClient: ts.Client(),
		URL:        ts.URL,
		Username:   "dummyUser",
		AccessKey:  "dummyAccesKey",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.RunTagAsync(tt.args.ctx, tt.args.hookID, tt.args.tagID, tt.args.buildID, tt.args.tunnel)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunAllAsync() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RunAllAsync() got = %v, want %v", got, tt.want)
			}
		})
	}
}
