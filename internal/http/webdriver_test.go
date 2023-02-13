package http

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/saucelabs/saucectl/internal/job"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type ResponseRecord struct {
	Index   int
	Records []func(w http.ResponseWriter, r *http.Request)
	Test    *testing.T
}

func (r *ResponseRecord) Record(resFunc func(w http.ResponseWriter, req *http.Request)) {
	r.Records = append(r.Records, resFunc)
}

func (r *ResponseRecord) Play(w http.ResponseWriter, req *http.Request) {
	if r.Index >= len(r.Records) {
		r.Test.Errorf("responder requested more times than it has available records")
	}

	r.Records[r.Index](w, req)
	r.Index++
}

func TestClient_StartJob(t *testing.T) {
	rec := ResponseRecord{
		Test: t,
	}
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.Play(w, r)
	}))
	type args struct {
		ctx               context.Context
		jobStarterPayload job.StartOptions
	}
	type fields struct {
		HTTPClient *http.Client
		URL        string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       string
		wantErr    error
		serverFunc func(w http.ResponseWriter, r *http.Request) // what shall the mock server respond with
	}{
		{
			name: "Happy path",
			fields: fields{
				HTTPClient: mockServer.Client(),
				URL:        mockServer.URL,
			},
			args: args{
				ctx: context.TODO(),
				jobStarterPayload: job.StartOptions{
					User:        "fake-user",
					AccessKey:   "fake-access-key",
					BrowserName: "fake-browser-name",
					Name:        "fake-test-name",
					Framework:   "fake-framework",
					Build:       "fake-buildname",
					Tags:        nil,
				},
			},
			want:    "fake-job-id",
			wantErr: nil,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				resp := sessionStartResponse{
					SessionID: "fake-job-id",
				}
				respondJSON(w, resp, 201)
			},
		},
		{
			name: "Non 2xx status code",
			fields: fields{
				HTTPClient: mockServer.Client(),
				URL:        mockServer.URL,
			},
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    "",
			wantErr: fmt.Errorf("job start failed (401): go away"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(401)
				_, _ = w.Write([]byte("go away"))
			},
		},
		{
			name: "Unknown error",
			fields: fields{
				HTTPClient: mockServer.Client(),
				URL:        mockServer.URL,
			},
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: job.StartOptions{},
			},
			want:    "",
			wantErr: fmt.Errorf("job start failed (500): internal server error"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				_, err := w.Write([]byte("internal server error"))
				if err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Webdriver{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
			}

			rec.Record(tt.serverFunc)

			got, _, err := c.StartJob(tt.args.ctx, tt.args.jobStarterPayload)
			if (err != nil) && !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("StartJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StartJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func respondJSON(w http.ResponseWriter, v interface{}, httpStatus int) {
	w.WriteHeader(httpStatus)
	b, err := json.Marshal(v)

	if err != nil {
		log.Err(err).Msg("failed to marshal job json")
		http.Error(w, "failed to marshal job json", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(b); err != nil {
		log.Err(err).Msg("Failed to write out response")
	}
}
