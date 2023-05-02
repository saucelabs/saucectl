package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/saucelabs/saucectl/internal/iam"
	"github.com/saucelabs/saucectl/internal/insights"
)

func TestInsightsService_PostTestRun(t *testing.T) {
	tests := []struct {
		name    string
		runs    []insights.TestRun
		reply   func(t *testing.T) func(w http.ResponseWriter, r *http.Request)
		wantErr bool
	}{
		{
			name: "Basic - empty - 204",
			runs: []insights.TestRun{},
			reply: func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(204)
				}
			},
			wantErr: false,
		},
		{
			name: "Basic - Erroring - 422",
			runs: []insights.TestRun{
				{
					ID: "09a87dea-3923-43db-8743-ef1f3ff5d717",
				},
			},
			reply: func(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(204)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		ts := httptest.NewServer(http.HandlerFunc(tt.reply(t)))

		t.Run(tt.name, func(t *testing.T) {
			c := &InsightsService{
				HTTPClient:  ts.Client(),
				URL:         ts.URL,
				Credentials: iam.Credentials{AccessKey: "accessKey", Username: "username"},
			}
			if err := c.PostTestRun(context.Background(), tt.runs); (err != nil) != tt.wantErr {
				t.Errorf("PostTestRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})

		ts.Close()
	}
}

func Test_mergeJobHistories(t *testing.T) {
	tests := []struct {
		name      string
		histories []insights.JobHistory
		want      insights.JobHistory
	}{
		{
			name:      "Empty Set",
			histories: []insights.JobHistory{},
			want:      insights.JobHistory{},
		},
		{
			name: "Single Set",
			histories: []insights.JobHistory{
				{
					TestCases: []insights.TestCase{
						{
							Name:     "suite1",
							FailRate: 0.15,
						},
						{
							Name:     "suite2",
							FailRate: 0.60,
						},
						{
							Name:     "suite3",
							FailRate: 0.30,
						},
						{
							Name:     "suite4",
							FailRate: 0.10,
						},
					},
				},
			},
			want: insights.JobHistory{
				TestCases: []insights.TestCase{
					{
						Name:     "suite2",
						FailRate: 0.60,
					},
					{
						Name:     "suite3",
						FailRate: 0.30,
					},
					{
						Name:     "suite1",
						FailRate: 0.15,
					},
					{
						Name:     "suite4",
						FailRate: 0.10,
					},
				},
			},
		},
		{
			name: "Multiple Set",
			histories: []insights.JobHistory{
				{
					TestCases: []insights.TestCase{
						{
							Name:     "suite11",
							FailRate: 0.15,
						},
						{
							Name:     "suite12",
							FailRate: 0.60,
						},
						{
							Name:     "suite13",
							FailRate: 0.30,
						},
						{
							Name:     "suite14",
							FailRate: 0.10,
						},
					},
				},
				{
					TestCases: []insights.TestCase{
						{
							Name:     "suite21",
							FailRate: 0.28,
						},
						{
							Name:     "suite22",
							FailRate: 0.34,
						},
						{
							Name:     "suite23",
							FailRate: 0.12,
						},
						{
							Name:     "suite24",
							FailRate: 0.68,
						},
					},
				},
			},
			want: insights.JobHistory{
				TestCases: []insights.TestCase{
					{
						Name:     "suite24",
						FailRate: 0.68,
					},
					{
						Name:     "suite12",
						FailRate: 0.60,
					},
					{
						Name:     "suite22",
						FailRate: 0.34,
					},
					{
						Name:     "suite13",
						FailRate: 0.30,
					},
					{
						Name:     "suite21",
						FailRate: 0.28,
					},
					{
						Name:     "suite11",
						FailRate: 0.15,
					},
					{
						Name:     "suite23",
						FailRate: 0.12,
					},
					{
						Name:     "suite14",
						FailRate: 0.10,
					},
				},
			},
		},
		{
			name: "Multiple Set - With Collisions",
			histories: []insights.JobHistory{
				{
					TestCases: []insights.TestCase{
						{
							Name:     "suite11",
							FailRate: 0.15,
						},
						{
							Name:     "suite12",
							FailRate: 0.60,
						},
						{
							Name:     "suite13",
							FailRate: 0.30,
						},
						{
							Name:     "suite14",
							FailRate: 0.10,
						},
						{
							Name:     "suite05",
							FailRate: 0.12,
						},
					},
				},
				{
					TestCases: []insights.TestCase{
						{
							Name:     "suite21",
							FailRate: 0.28,
						},
						{
							Name:     "suite22",
							FailRate: 0.34,
						},
						{
							Name:     "suite23",
							FailRate: 0.12,
						},
						{
							Name:     "suite24",
							FailRate: 0.68,
						},
						{
							Name:     "suite05",
							FailRate: 0.35,
						},
					},
				},
			},
			want: insights.JobHistory{
				TestCases: []insights.TestCase{
					{
						Name:     "suite24",
						FailRate: 0.68,
					},
					{
						Name:     "suite12",
						FailRate: 0.60,
					},
					{
						Name:     "suite05",
						FailRate: 0.35,
					},
					{
						Name:     "suite22",
						FailRate: 0.34,
					},
					{
						Name:     "suite13",
						FailRate: 0.30,
					},
					{
						Name:     "suite21",
						FailRate: 0.28,
					},
					{
						Name:     "suite11",
						FailRate: 0.15,
					},
					{
						Name:     "suite23",
						FailRate: 0.12,
					},
					{
						Name:     "suite14",
						FailRate: 0.10,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, mergeJobHistories(tt.histories), "mergeJobHistories(%v)", tt.histories)
		})
	}
}
