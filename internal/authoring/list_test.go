package authoring

import (
	"context"
	"errors"
	"testing"
)

// pagedServer fakes a listing endpoint over a fixed set of integers.
func pagedServer(total int) func(ctx context.Context, opts ListOptions) (List[int], error) {
	return func(_ context.Context, opts ListOptions) (List[int], error) {
		var items []int
		for i := opts.Skip; i < total && i < opts.Skip+*opts.Limit; i++ {
			items = append(items, i)
		}
		return List[int]{Items: items, Total: total}, nil
	}
}

func TestListAll(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		pageSize  int
		wantCalls int
	}{
		{name: "exact multiple of the page size", total: 200, pageSize: 100, wantCalls: 2},
		{name: "partial last page", total: 150, pageSize: 100, wantCalls: 2},
		{name: "fewer than one page", total: 7, pageSize: 100, wantCalls: 1},
		{name: "zero total", total: 0, pageSize: 100, wantCalls: 1},
		{name: "non-positive page size uses the default", total: DefaultPageSize + 1, pageSize: 0, wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := pagedServer(tt.total)
			fetch := func(ctx context.Context, opts ListOptions) (List[int], error) {
				calls++
				if opts.Limit == nil {
					t.Fatal("ListAll must always send a limit")
				}
				return srv(ctx, opts)
			}
			got, err := ListAll(context.Background(), tt.pageSize, fetch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.total {
				t.Errorf("got %d items, want %d", len(got), tt.total)
			}
			for i, v := range got {
				if v != i {
					t.Fatalf("item %d = %d; pages were not disjoint or not ordered", i, v)
				}
			}
			if calls != tt.wantCalls {
				t.Errorf("fetch called %d times, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestListAll_NeverEndingServer(t *testing.T) {
	// A server that ignores skip and always returns a full page with a huge
	// total must not loop forever.
	calls := 0
	fetch := func(_ context.Context, opts ListOptions) (List[int], error) {
		calls++
		items := make([]int, *opts.Limit)
		return List[int]{Items: items, Total: 1 << 30}, nil
	}
	_, err := ListAll(context.Background(), 10, fetch)
	if !errors.Is(err, ErrListTooLong) {
		t.Fatalf("expected ErrListTooLong, got %v", err)
	}
	if calls != maxListPages {
		t.Errorf("fetch called %d times, want exactly %d", calls, maxListPages)
	}
}

func TestListAll_PropagatesFetchError(t *testing.T) {
	boom := errors.New("boom")
	calls := 0
	fetch := func(_ context.Context, opts ListOptions) (List[int], error) {
		calls++
		if calls == 2 {
			return List[int]{}, boom
		}
		return List[int]{Items: make([]int, *opts.Limit), Total: 1000}, nil
	}
	got, err := ListAll(context.Background(), 10, fetch)
	if !errors.Is(err, boom) {
		t.Fatalf("expected the fetch error, got %v", err)
	}
	if len(got) != 10 {
		t.Errorf("expected the first page to be returned alongside the error, got %d items", len(got))
	}
}
