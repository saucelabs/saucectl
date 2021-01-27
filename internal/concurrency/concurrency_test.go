package concurrency

import (
	"context"
	"errors"
	"testing"
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
