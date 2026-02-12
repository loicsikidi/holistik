package memory

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/loicsikidi/holistik/internal/apiserver/lib/backend"
	"github.com/loicsikidi/sentinel"
)

func TestConfig_CheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want Config
	}{
		{
			name: "all defaults",
			cfg:  Config{},
			want: Config{
				BTreeDegree: 8,
				MaxItemSize: 1048576,
			},
		},
		{
			name: "custom values preserved",
			cfg: Config{
				BTreeDegree: 16,
				MaxItemSize: 2097152,
			},
			want: Config{
				BTreeDegree: 16,
				MaxItemSize: 2097152,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.CheckAndSetDefaults()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.cfg.BTreeDegree != tt.want.BTreeDegree {
				t.Errorf("BTreeDegree = %d, want %d", tt.cfg.BTreeDegree, tt.want.BTreeDegree)
			}
			if tt.cfg.MaxItemSize != tt.want.MaxItemSize {
				t.Errorf("MaxItemSize = %d, want %d", tt.cfg.MaxItemSize, tt.want.MaxItemSize)
			}
			if tt.cfg.Context == nil {
				t.Error("Context should not be nil after CheckAndSetDefaults")
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		b, err := New(Config{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b == nil {
			t.Fatal("expected non-nil Memory")
		}
		if b.GetName() != "memory" {
			t.Errorf("GetName() = %q, want %q", b.GetName(), "memory")
		}
	})
}

func TestMemory_Create(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend)
		item     backend.Item
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "create new item",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("data"),
			},
			wantErr: false,
		},
		{
			name: "already exists - non expired",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("existing"),
				})
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			},
			wantErr:  true,
			errCheck: sentinel.IsAlreadyExists,
		},
		{
			name: "replace expired item",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("old"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			},
			wantErr: false,
		},
		{
			name: "exceeds max size",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: make([]byte, 2*1024*1024), // 2MB
			},
			wantErr:  true,
			errCheck: sentinel.IsBadParameter,
		},
		{
			name: "with expiration",
			item: backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("data"),
				Expires: time.Now().Add(1 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "generates revision if missing",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("data"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			if tt.setup != nil {
				tt.setup(b)
			}

			lease, err := b.Create(ctx, tt.item)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if lease == nil {
				t.Fatal("expected non-nil lease")
			}

			if lease.Revision == "" {
				t.Error("expected non-empty revision")
			}
		})
	}
}

func TestMemory_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend)
		key      backend.Key
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "get existing item",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("data"),
				})
			},
			key:     backend.NewKey("test"),
			wantErr: false,
		},
		{
			name:     "not found",
			key:      backend.NewKey("missing"),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "expired item returns not found",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("data"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
			},
			key:      backend.NewKey("test"),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			if tt.setup != nil {
				tt.setup(b)
			}

			item, err := b.Get(ctx, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if item == nil {
				t.Fatal("expected non-nil item")
			}
		})
	}
}

func TestMemory_Delete(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend)
		key      backend.Key
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "delete existing",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("data"),
				})
			},
			key:     backend.NewKey("test"),
			wantErr: false,
		},
		{
			name:     "not found",
			key:      backend.NewKey("missing"),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "expired returns not found",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("data"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
			},
			key:      backend.NewKey("test"),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			if tt.setup != nil {
				tt.setup(b)
			}

			err = b.Delete(ctx, tt.key)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify item is actually deleted
			_, err = b.Get(ctx, tt.key)
			if !sentinel.IsNotFound(err) {
				t.Error("item should be deleted")
			}
		})
	}
}

func TestMemory_Items(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(backend.Backend)
		params    backend.ItemsParams
		wantKeys  []string
		wantCount int
	}{
		{
			name: "all items ascending",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("b"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
			},
			params:    backend.ItemsParams{},
			wantKeys:  []string{"/a", "/b", "/c"},
			wantCount: 3,
		},
		{
			name: "descending order",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("b"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
			},
			params: backend.ItemsParams{
				Descending: true,
			},
			wantKeys:  []string{"/c", "/b", "/a"},
			wantCount: 3,
		},
		{
			name: "range query",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("b"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("d"), Value: []byte("4")})
			},
			params: backend.ItemsParams{
				StartKey: backend.NewKey("b"),
				EndKey:   backend.NewKey("c"),
			},
			wantKeys:  []string{"/b", "/c"},
			wantCount: 2,
		},
		{
			name: "with limit",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("b"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
			},
			params: backend.ItemsParams{
				Limit: 2,
			},
			wantKeys:  []string{"/a", "/b"},
			wantCount: 2,
		},
		{
			name: "skip expired items",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{
					Key:     backend.NewKey("b"),
					Value:   []byte("2"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
			},
			params:    backend.ItemsParams{},
			wantKeys:  []string{"/a", "/c"},
			wantCount: 2,
		},
		{
			name: "prefix range with ExactKey",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("users", "alice"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("users", "bob"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("devices", "x"), Value: []byte("3")})
			},
			params: backend.ItemsParams{
				StartKey: backend.ExactKey("users"),
				EndKey:   backend.RangeEnd(backend.ExactKey("users")),
			},
			wantKeys:  []string{"/users/alice", "/users/bob"},
			wantCount: 2,
		},
		{
			name: "descending with limit",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{Key: backend.NewKey("a"), Value: []byte("1")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("b"), Value: []byte("2")})
				b.Create(ctx, backend.Item{Key: backend.NewKey("c"), Value: []byte("3")})
			},
			params: backend.ItemsParams{
				Descending: true,
				Limit:      2,
			},
			wantKeys:  []string{"/c", "/b"},
			wantCount: 2,
		},
		{
			name:      "empty range",
			params:    backend.ItemsParams{},
			wantKeys:  []string{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			if tt.setup != nil {
				tt.setup(b)
			}

			var gotKeys []string
			count := 0
			for item, err := range b.Items(ctx, tt.params) {
				if err != nil {
					t.Fatalf("unexpected error during iteration: %v", err)
				}
				gotKeys = append(gotKeys, item.Key.String())
				count++
			}

			if count != tt.wantCount {
				t.Errorf("got %d items, want %d", count, tt.wantCount)
			}

			if len(gotKeys) != len(tt.wantKeys) {
				t.Errorf("got keys %v, want %v", gotKeys, tt.wantKeys)
				return
			}

			for i, key := range gotKeys {
				if key != tt.wantKeys[i] {
					t.Errorf("key[%d] = %q, want %q", i, key, tt.wantKeys[i])
				}
			}
		})
	}
}

func TestMemory_Items_ContextCancellation(t *testing.T) {
	b, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Create many items
	for i := 0; i < 100; i++ {
		b.Create(context.Background(), backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("item%03d", i)),
			Value: []byte("data"),
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	count := 0
	gotContextError := false
	for _, err := range b.Items(ctx, backend.ItemsParams{}) {
		if err != nil {
			// Expected: wrapped context cancellation error
			gotContextError = true
			break
		}
		count++
	}

	// We should get a context error since we cancelled immediately
	if !gotContextError && count >= 100 {
		t.Error("expected context cancellation error or early termination")
	}
}

func TestMemory_Concurrency(t *testing.T) {
	b, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 10
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 6) // Create, Get, Delete, Put, Update, KeepAlive

	ctx := context.Background()

	// Concurrent Creates
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-item%d", id, j))
				b.Create(ctx, backend.Item{
					Key:   key,
					Value: []byte(fmt.Sprintf("data-%d-%d", id, j)),
				})
			}
		}(i)
	}

	// Concurrent Gets
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-item%d", id, j))
				b.Get(ctx, key)
			}
		}(i)
	}

	// Concurrent Puts
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-put%d", id, j))
				b.Put(ctx, backend.Item{
					Key:   key,
					Value: []byte(fmt.Sprintf("put-%d-%d", id, j)),
				})
			}
		}(i)
	}

	// Concurrent Updates
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-item%d", id, j))
				b.Update(ctx, backend.Item{
					Key:   key,
					Value: []byte(fmt.Sprintf("updated-%d-%d", id, j)),
				})
			}
		}(i)
	}

	// Concurrent KeepAlives
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-item%d", id, j))
				b.KeepAlive(ctx, backend.Lease{Key: key}, time.Now().Add(1*time.Hour))
			}
		}(i)
	}

	// Concurrent Deletes
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for j := range iterations {
				key := backend.NewKey(fmt.Sprintf("g%d-item%d", id, j))
				b.Delete(ctx, key)
			}
		}(i)
	}

	wg.Wait()
	// No race conditions = success
}

func TestMemory_Put(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend)
		item     backend.Item
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "put new item",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("data"),
			},
			wantErr: false,
		},
		{
			name: "put overwrites existing non-expired",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			},
			wantErr: false,
		},
		{
			name: "put overwrites expired",
			setup: func(b backend.Backend) {
				b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("old"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			},
			wantErr: false,
		},
		{
			name: "put exceeds max size",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: make([]byte, 2*1024*1024),
			},
			wantErr:  true,
			errCheck: sentinel.IsBadParameter,
		},
		{
			name: "put generates revision if missing",
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("data"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			if tt.setup != nil {
				tt.setup(b)
			}

			lease, err := b.Put(ctx, tt.item)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if lease == nil {
				t.Fatal("expected non-nil lease")
			}

			if lease.Revision == "" {
				t.Error("expected non-empty revision")
			}

			// Verify item was actually stored
			item, err := b.Get(ctx, tt.item.Key)
			if err != nil {
				t.Fatalf("failed to get item after Put: %v", err)
			}

			if string(item.Value) != string(tt.item.Value) {
				t.Errorf("value = %q, want %q", string(item.Value), string(tt.item.Value))
			}
		})
	}
}

func TestMemory_Update(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend) string // Returns revision of created item
		item     backend.Item
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "update existing item with revision",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
				// Revision will be set in test body
			},
			wantErr: false,
		},
		{
			name: "update non-existent item",
			item: backend.Item{
				Key:   backend.NewKey("missing"),
				Value: []byte("data"),
			},
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "update expired item",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("old"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			},
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "update with correct revision",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
				// Revision will be set in test body
			},
			wantErr: false,
		},
		{
			name: "update with wrong revision",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:      backend.NewKey("test"),
				Value:    []byte("new"),
				Revision: "wrong-revision",
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return sentinel.IsCompareFailed(err) && err == backend.ErrIncorrectRevision
			},
		},
		{
			name: "update with empty revision",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:      backend.NewKey("test"),
				Value:    []byte("new"),
				Revision: "", // Empty revision should fail
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return sentinel.IsCompareFailed(err) && err == backend.ErrIncorrectRevision
			},
		},
		{
			name: "update exceeds max size",
			setup: func(b backend.Backend) string {
				lease, _ := b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("old"),
				})
				return lease.Revision
			},
			item: backend.Item{
				Key:   backend.NewKey("test"),
				Value: make([]byte, 2*1024*1024),
			},
			wantErr:  true,
			errCheck: sentinel.IsBadParameter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			var originalRevision string
			if tt.setup != nil {
				originalRevision = tt.setup(b)
			}

			// Special case: tests that need the correct revision
			if tt.name == "update with correct revision" || tt.name == "update existing item with revision" {
				tt.item.Revision = originalRevision
			}

			lease, err := b.Update(ctx, tt.item)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if lease == nil {
				t.Fatal("expected non-nil lease")
			}

			if lease.Revision == "" {
				t.Error("expected non-empty revision")
			}

			// Verify new revision is different from original
			if (tt.name == "update with correct revision" || tt.name == "update existing item with revision") && lease.Revision == originalRevision {
				t.Error("expected new revision after update")
			}

			// Verify item was actually updated
			item, err := b.Get(ctx, tt.item.Key)
			if err != nil {
				t.Fatalf("failed to get item after Update: %v", err)
			}

			if string(item.Value) != string(tt.item.Value) {
				t.Errorf("value = %q, want %q", string(item.Value), string(tt.item.Value))
			}
		})
	}
}

func TestMemory_KeepAlive(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(backend.Backend) backend.Lease
		expires  time.Time
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "keep alive existing item",
			setup: func(b backend.Backend) backend.Lease {
				lease, _ := b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("data"),
					Expires: time.Now().Add(1 * time.Hour),
				})
				return *lease
			},
			expires: time.Now().Add(2 * time.Hour),
			wantErr: false,
		},
		{
			name: "keep alive non-existent item",
			setup: func(b backend.Backend) backend.Lease {
				return backend.Lease{
					Key:      backend.NewKey("missing"),
					Revision: "fake",
				}
			},
			expires:  time.Now().Add(1 * time.Hour),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "keep alive expired item",
			setup: func(b backend.Backend) backend.Lease {
				lease, _ := b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("data"),
					Expires: time.Now().Add(-1 * time.Hour),
				})
				return *lease
			},
			expires:  time.Now().Add(1 * time.Hour),
			wantErr:  true,
			errCheck: sentinel.IsNotFound,
		},
		{
			name: "keep alive with wrong revision",
			setup: func(b backend.Backend) backend.Lease {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("data"),
				})
				return backend.Lease{
					Key:      backend.NewKey("test"),
					Revision: "wrong",
				}
			},
			expires: time.Now().Add(1 * time.Hour),
			wantErr: true,
			errCheck: func(err error) bool {
				return sentinel.IsCompareFailed(err) && err == backend.ErrIncorrectRevision
			},
		},
		{
			name: "keep alive with empty revision",
			setup: func(b backend.Backend) backend.Lease {
				b.Create(ctx, backend.Item{
					Key:   backend.NewKey("test"),
					Value: []byte("data"),
				})
				return backend.Lease{
					Key:      backend.NewKey("test"),
					Revision: "", // Empty revision should fail
				}
			},
			expires: time.Now().Add(1 * time.Hour),
			wantErr: true,
			errCheck: func(err error) bool {
				return sentinel.IsCompareFailed(err) && err == backend.ErrIncorrectRevision
			},
		},
		{
			name: "keep alive extends expiration",
			setup: func(b backend.Backend) backend.Lease {
				lease, _ := b.Create(ctx, backend.Item{
					Key:     backend.NewKey("test"),
					Value:   []byte("data"),
					Expires: time.Now().Add(10 * time.Second),
				})
				return *lease
			},
			expires: time.Now().Add(1 * time.Hour),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatalf("failed to create Memory: %v", err)
			}

			lease := tt.setup(b)

			err = b.KeepAlive(ctx, lease, tt.expires)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errCheck != nil && !tt.errCheck(err) {
					t.Errorf("error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify expiration was updated
			item, err := b.Get(ctx, lease.Key)
			if err != nil {
				t.Fatalf("failed to get item after KeepAlive: %v", err)
			}

			// Check that expires time is reasonably close to what we set
			// (allowing for small time differences due to test execution)
			if item.Expires.Sub(tt.expires).Abs() > time.Second {
				t.Errorf("expires = %v, want ~%v", item.Expires, tt.expires)
			}
		})
	}
}

func TestMemory_Close(t *testing.T) {
	b, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}

	err = b.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestMemory_ExpirationWithSynctest tests expiration behavior using synctest
// to control time advancement without relying on actual time passage.
func TestMemory_ExpirationWithSynctest(t *testing.T) {
	t.Run("create item that expires", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with 1 hour expiration
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("expiring"),
				Value:   []byte("data"),
				Expires: now.Add(1 * time.Hour),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Item should exist now
			item, err := b.Get(ctx, backend.NewKey("expiring"))
			if err != nil {
				t.Fatalf("item should exist: %v", err)
			}
			if string(item.Value) != "data" {
				t.Errorf("value = %q, want %q", string(item.Value), "data")
			}

			// Advance time past expiration
			synctest.Wait()
			time.Sleep(2 * time.Hour)

			// Item should now be expired and return NotFound
			_, err = b.Get(ctx, backend.NewKey("expiring"))
			if !sentinel.IsNotFound(err) {
				t.Errorf("expected NotFound after expiration, got: %v", err)
			}
		})
	})

	t.Run("create overwrites expired item", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with short expiration
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("old"),
				Expires: now.Add(30 * time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Advance time past expiration
			synctest.Wait()
			time.Sleep(1 * time.Hour)

			// Create should succeed (overwrites expired item)
			_, err = b.Create(ctx, backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			})
			if err != nil {
				t.Fatalf("create should succeed on expired item: %v", err)
			}

			// Verify new value
			item, err := b.Get(ctx, backend.NewKey("test"))
			if err != nil {
				t.Fatal(err)
			}
			if string(item.Value) != "new" {
				t.Errorf("value = %q, want %q", string(item.Value), "new")
			}
		})
	})

	t.Run("update fails on expired item", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with short expiration
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("old"),
				Expires: now.Add(30 * time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Advance time past expiration
			synctest.Wait()
			time.Sleep(1 * time.Hour)

			// Update should fail with NotFound
			_, err = b.Update(ctx, backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			})
			if !sentinel.IsNotFound(err) {
				t.Errorf("expected NotFound for expired item, got: %v", err)
			}
		})
	})

	t.Run("keep alive extends expiration", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with 1 hour expiration
			lease, err := b.Create(ctx, backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("data"),
				Expires: now.Add(1 * time.Hour),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Advance time by 30 minutes (not expired yet)
			synctest.Wait()
			time.Sleep(30 * time.Minute)

			// Item should still exist
			_, err = b.Get(ctx, backend.NewKey("test"))
			if err != nil {
				t.Fatalf("item should still exist: %v", err)
			}

			// Extend expiration by another 2 hours from current time
			newExpires := time.Now().Add(2 * time.Hour)
			err = b.KeepAlive(ctx, *lease, newExpires)
			if err != nil {
				t.Fatalf("KeepAlive failed: %v", err)
			}

			// Advance time by 90 minutes (would be expired without KeepAlive)
			synctest.Wait()
			time.Sleep(90 * time.Minute)

			// Item should still exist due to KeepAlive
			item, err := b.Get(ctx, backend.NewKey("test"))
			if err != nil {
				t.Fatalf("item should exist after KeepAlive: %v", err)
			}
			if string(item.Value) != "data" {
				t.Errorf("value = %q, want %q", string(item.Value), "data")
			}
		})
	})

	t.Run("items skips expired entries", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create multiple items with different expirations
			b.Create(ctx, backend.Item{
				Key:   backend.NewKey("permanent"),
				Value: []byte("1"),
			})
			b.Create(ctx, backend.Item{
				Key:     backend.NewKey("short"),
				Value:   []byte("2"),
				Expires: now.Add(30 * time.Minute),
			})
			b.Create(ctx, backend.Item{
				Key:     backend.NewKey("long"),
				Value:   []byte("3"),
				Expires: now.Add(2 * time.Hour),
			})

			// Advance time by 45 minutes (expires "short" but not "long")
			synctest.Wait()
			time.Sleep(45 * time.Minute)

			// Iterate and count non-expired items
			var keys []string
			for item, err := range b.Items(ctx, backend.ItemsParams{}) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				keys = append(keys, item.Key.String())
			}

			// Should only see "permanent" and "long", not "short"
			if len(keys) != 2 {
				t.Errorf("expected 2 items, got %d: %v", len(keys), keys)
			}

			expectedKeys := map[string]bool{
				"/permanent": true,
				"/long":      true,
			}
			for _, key := range keys {
				if !expectedKeys[key] {
					t.Errorf("unexpected key %q", key)
				}
			}
		})
	})

	t.Run("delete fails on expired item", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with short expiration
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("data"),
				Expires: now.Add(30 * time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Advance time past expiration
			synctest.Wait()
			time.Sleep(1 * time.Hour)

			// Delete should fail with NotFound
			err = b.Delete(ctx, backend.NewKey("test"))
			if !sentinel.IsNotFound(err) {
				t.Errorf("expected NotFound for expired item, got: %v", err)
			}
		})
	})

	t.Run("put works on expired item", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create item with short expiration
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("test"),
				Value:   []byte("old"),
				Expires: now.Add(30 * time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Advance time past expiration
			synctest.Wait()
			time.Sleep(1 * time.Hour)

			// Put should succeed (overwrites expired item)
			_, err = b.Put(ctx, backend.Item{
				Key:   backend.NewKey("test"),
				Value: []byte("new"),
			})
			if err != nil {
				t.Fatalf("put should succeed on expired item: %v", err)
			}

			// Verify new value
			item, err := b.Get(ctx, backend.NewKey("test"))
			if err != nil {
				t.Fatal(err)
			}
			if string(item.Value) != "new" {
				t.Errorf("value = %q, want %q", string(item.Value), "new")
			}
		})
	})

	t.Run("expired items are physically removed from cache", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			b, err := New(Config{})
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			now := time.Now()

			// Create multiple items with different expirations
			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("expires-soon"),
				Value:   []byte("data1"),
				Expires: now.Add(30 * time.Minute),
			})
			if err != nil {
				t.Fatal(err)
			}

			_, err = b.Create(ctx, backend.Item{
				Key:     backend.NewKey("expires-later"),
				Value:   []byte("data2"),
				Expires: now.Add(2 * time.Hour),
			})
			if err != nil {
				t.Fatal(err)
			}

			_, err = b.Create(ctx, backend.Item{
				Key:   backend.NewKey("permanent"),
				Value: []byte("data3"),
			})
			if err != nil {
				t.Fatal(err)
			}

			// Verify initial tree size
			mem := b.(*Memory)
			initialSize := mem.tree.Len()
			if initialSize != 3 {
				t.Fatalf("expected 3 items in tree, got %d", initialSize)
			}

			// Advance time to expire first item
			synctest.Wait()
			time.Sleep(45 * time.Minute)

			// Trigger cleanup by calling any method (using Get)
			b.Get(ctx, backend.NewKey("permanent"))

			// Verify that expired item was physically removed
			afterCleanupSize := mem.tree.Len()
			if afterCleanupSize != 2 {
				t.Errorf("expected 2 items in tree after cleanup, got %d", afterCleanupSize)
			}

			// Advance time to expire second item
			synctest.Wait()
			time.Sleep(2 * time.Hour)

			// Trigger cleanup again
			b.Get(ctx, backend.NewKey("permanent"))

			// Verify that only permanent item remains
			finalSize := mem.tree.Len()
			if finalSize != 1 {
				t.Errorf("expected 1 item in tree after second cleanup, got %d", finalSize)
			}

			// Verify the remaining item is the permanent one
			item, err := b.Get(ctx, backend.NewKey("permanent"))
			if err != nil {
				t.Fatalf("permanent item should still exist: %v", err)
			}
			if string(item.Value) != "data3" {
				t.Errorf("value = %q, want %q", string(item.Value), "data3")
			}

			// Verify expired items return NotFound
			_, err = b.Get(ctx, backend.NewKey("expires-soon"))
			if !sentinel.IsNotFound(err) {
				t.Errorf("expires-soon should return NotFound, got: %v", err)
			}

			_, err = b.Get(ctx, backend.NewKey("expires-later"))
			if !sentinel.IsNotFound(err) {
				t.Errorf("expires-later should return NotFound, got: %v", err)
			}
		})
	})
}
