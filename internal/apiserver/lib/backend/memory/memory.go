package memory

import (
	"context"
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/loicsikidi/holistik/internal/apiserver/lib/backend"
	"github.com/loicsikidi/sentinel"
	"github.com/tidwall/btree"
)

const (
	defaultMaxItemSize = 1048576 // 1MB
	defaultBTreeDegree = 8
)

// Config holds configuration for the memory backend.
type Config struct {
	// Context is a context for opening the database.
	//
	// Optional.
	// Default is context.Background().
	Context context.Context
	// BTreeDegree is the degree of the B-Tree, 2 for example, will create a
	// 2-3-4 tree (each node contains 1-3 items and 2-4 children).
	//
	// Optional.
	// Default is 8.
	BTreeDegree int
	// MaxItemSize sets up the maximum size of an item in bytes.
	//
	// Optional.
	// Default is 1048576 (1MB).
	MaxItemSize int
}

// CheckAndSetDefaults validates and sets default values for the config.
func (c *Config) CheckAndSetDefaults() error {
	if c.BTreeDegree <= 0 {
		c.BTreeDegree = defaultBTreeDegree
	}
	if c.MaxItemSize <= 0 {
		c.MaxItemSize = defaultMaxItemSize
	}
	if c.Context == nil {
		c.Context = context.Background()
	}
	return nil
}

// storedItem wraps a [backend.Item] for storage in the B-tree.
type storedItem struct {
	key  string
	item backend.Item
}

// Memory is an in-memory backend implementation using a B-tree.
type Memory struct {
	mu   sync.RWMutex
	tree *btree.BTreeG[storedItem]
	cfg  Config
}

// New creates a new memory backend with the given configuration.
func New(cfg Config) (backend.Backend, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, sentinel.Wrap(err)
	}

	return &Memory{
		tree: btree.NewBTreeGOptions(
			func(a, b storedItem) bool { return a.key < b.key },
			btree.Options{Degree: cfg.BTreeDegree},
		),
		cfg: cfg,
	}, nil
}

// GetName returns the backend implementation name.
// This value is "memory" is expected to be used in storage section of the configuration YALM file.
func (m *Memory) GetName() string {
	return "memory"
}

// Close closes the backend and releases resources.
func (m *Memory) Close() error {
	return nil // Nothing to cleanup for in-memory backend
}

// validateItemSize validates that the item size doesn't exceed the configured maximum.
func (m *Memory) validateItemSize(i backend.Item) error {
	if len(i.Value) > m.cfg.MaxItemSize {
		return sentinel.BadParameter("item size %d exceeds maximum %d",
			len(i.Value), m.cfg.MaxItemSize)
	}
	return nil
}

// cleanupExpiredItems removes all expired items from the tree.
//
// Note: this method assumes the caller holds the write lock.
func (m *Memory) cleanupExpiredItems() {
	var keysToDelete []string

	// Collect expired keys
	m.tree.Scan(func(item storedItem) bool {
		if isExpired(item.item) {
			keysToDelete = append(keysToDelete, item.key)
		}
		return true
	})

	// Delete expired items
	for _, key := range keysToDelete {
		m.tree.Delete(storedItem{key: key})
	}
}

// Create creates an item if it does not exist.
func (m *Memory) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if err := m.validateItemSize(i); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredItems()

	key := i.Key.String()

	existing, found := m.tree.Get(storedItem{key: key})
	if found && !isExpired(existing.item) {
		return nil, sentinel.AlreadyExists("item with key %q already exists", key)
	}

	if i.Revision == "" {
		i.Revision = backend.CreateRevision()
	}

	m.tree.Set(storedItem{key: key, item: i})

	return &backend.Lease{
		Key:      i.Key,
		Revision: i.Revision,
	}, nil
}

// Get returns a single item or [sentinel.NotFound] error.
func (m *Memory) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
	m.mu.Lock()
	m.cleanupExpiredItems()
	m.mu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	stored, found := m.tree.Get(storedItem{key: key.String()})
	if !found || isExpired(stored.item) {
		return nil, sentinel.NotFound("item with key %q not found", key.String())
	}

	// Return copy to prevent external modifications
	copy := stored.item
	return &copy, nil
}

// Delete deletes an item by key.
func (m *Memory) Delete(ctx context.Context, key backend.Key) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredItems()

	stored, found := m.tree.Get(storedItem{key: key.String()})
	if !found || isExpired(stored.item) {
		return sentinel.NotFound("item with key %q not found", key.String())
	}

	m.tree.Delete(storedItem{key: key.String()})
	return nil
}

// Items produces an iterator of backend items in the range described by params.
func (m *Memory) Items(ctx context.Context, params backend.ItemsParams) iter.Seq2[backend.Item, error] {
	// Build snapshot under lock to ensure consistency
	m.mu.Lock()
	m.cleanupExpiredItems()
	m.mu.Unlock()

	m.mu.RLock()

	startKey := params.StartKey.String()
	endKey := params.EndKey.String()

	var snapshot []storedItem

	// Collect items in range
	m.tree.Ascend(storedItem{key: startKey}, func(item storedItem) bool {
		// If endKey is specified and we've passed it, stop
		if endKey != "" && item.key > endKey {
			return false
		}
		snapshot = append(snapshot, item)
		return true
	})

	m.mu.RUnlock()

	// Apply descending order if requested
	if params.Descending {
		slices.Reverse(snapshot)
	}

	// Return iterator function
	return func(yield func(backend.Item, error) bool) {
		count := 0
		for _, stored := range snapshot {
			// Check context cancellation
			if err := ctx.Err(); err != nil {
				yield(backend.Item{}, sentinel.Wrap(err))
				return
			}

			// Skip expired items
			if isExpired(stored.item) {
				continue
			}

			// Check limit after filtering expired items
			if params.Limit > 0 && count >= params.Limit {
				return
			}

			if !yield(stored.item, nil) {
				return
			}
			count++
		}
	}
}

// Put puts value into backend (creates if it does not exist, updates it otherwise).
func (m *Memory) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if err := m.validateItemSize(i); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredItems()

	key := i.Key.String()

	// Generate revision if not provided
	if i.Revision == "" {
		i.Revision = backend.CreateRevision()
	}

	// Store item (overwrites if exists, even if not expired)
	m.tree.Set(storedItem{key: key, item: i})

	return &backend.Lease{
		Key:      i.Key,
		Revision: i.Revision,
	}, nil
}

// Update updates conditionally value in the backend.
func (m *Memory) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	if err := m.validateItemSize(i); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredItems()

	key := i.Key.String()

	// Check if item exists and is not expired
	existing, found := m.tree.Get(storedItem{key: key})
	if !found || isExpired(existing.item) {
		return nil, sentinel.NotFound("item with key %q not found", key)
	}

	// Check revision for optimistic locking - revision must always match
	if i.Revision != existing.item.Revision {
		return nil, backend.ErrIncorrectRevision
	}

	// Generate new revision
	i.Revision = backend.CreateRevision()

	// Update item
	m.tree.Set(storedItem{key: key, item: i})

	return &backend.Lease{
		Key:      i.Key,
		Revision: i.Revision,
	}, nil
}

// KeepAlive keeps object from expiring, updates lease on the existing object.
func (m *Memory) KeepAlive(ctx context.Context, lease backend.Lease, expires time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupExpiredItems()

	key := lease.Key.String()

	// Check if item exists and is not expired
	existing, found := m.tree.Get(storedItem{key: key})
	if !found || isExpired(existing.item) {
		return sentinel.NotFound("item with key %q not found", key)
	}

	// Check revision for optimistic locking - revision must always match
	if lease.Revision != existing.item.Revision {
		return backend.ErrIncorrectRevision
	}

	// Update only the Expires field
	existing.item.Expires = expires
	m.tree.Set(storedItem{key: key, item: existing.item})

	return nil
}

// isExpired returns true if the item has expired.
//
// An item is considered expired if:
//   - Expires is set (non-zero)
//   - and the current time is after the expiration time.
func isExpired(item backend.Item) bool {
	return !item.Expires.IsZero() && time.Now().After(item.Expires)
}
