package concurrency

import (
	"context"
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

// ccyReader is a mock implemention for the concurrency.Reader interface.
type ccyReader struct {
	ReadAllowedCCYfn func(ctx context.Context) (int, error)
}

// ReadAllowedCCY is a wrapper around ccyReader.ReadAllowedCCYfn.
func (r ccyReader) ReadAllowedCCY(ctx context.Context) (int, error) {
	return r.ReadAllowedCCYfn(ctx)
}

func TestMin(t *testing.T) {
	type args struct {
		r   Reader
		ccy int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "below the limit",
			args: args{
				r: ccyReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
					return 10, nil
				}},
				ccy: 5,
			},
			want: 5,
		},
		{
			name: "at the limit",
			args: args{
				r: ccyReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
					return 10, nil
				}},
				ccy: 10,
			},
			want: 10,
		},
		{
			name: "above the limit",
			args: args{
				r: ccyReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
					return 10, nil
				}},
				ccy: 20,
			},
			want: 10,
		},
		{
			name: "on error",
			args: args{
				r: ccyReader{ReadAllowedCCYfn: func(ctx context.Context) (int, error) {
					return 0, errors.New("better be expecting me")
				}},
				ccy: 20,
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Min(tt.args.r, tt.args.ccy); got != tt.want {
				t.Errorf("Min() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SplitTestFiles(t *testing.T) {
	var testCases = []struct {
		name      string
		files     []string
		count     int
		expResult [][]string
	}{
		{
			name:      "concuurrency is 1",
			files:     []string{"1", "2", "3"},
			count:     1,
			expResult: [][]string{[]string{"1", "2", "3"}},
		},
		{
			name:      "concuurrency is less than file count",
			files:     []string{"1", "2", "3"},
			count:     2,
			expResult: [][]string{[]string{"1"}, []string{"2", "3"}},
		},
		{
			name:      "file count can be devidec by concurrency",
			files:     []string{"1", "2", "3"},
			count:     3,
			expResult: [][]string{[]string{"1"}, []string{"2"}, []string{"3"}},
		},
		{
			name:      "concuurrency is greater than file count",
			files:     []string{"1", "2", "3"},
			count:     5,
			expResult: [][]string{[]string{"1"}, []string{"2"}, []string{"3"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SplitTestFiles(tc.files, tc.count)
			assert.Equal(t, len(tc.expResult), len(result))
			for i := 0; i < len(result); i++ {
				for j := 0; j < len(result[i]); j++ {
					assert.Equal(t, tc.expResult[i][j], result[i][j])
				}
			}
		})
	}
}
