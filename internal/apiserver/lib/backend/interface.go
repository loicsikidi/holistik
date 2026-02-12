package backend

import (
	"context"
	"iter"
	"time"

	"github.com/loicsikidi/sentinel"
)

// ErrIncorrectRevision is returned from conditional operations when revisions
// do not match the expected value.
var ErrIncorrectRevision = sentinel.CompareFailed("resource revision does not match, it may have been concurrently created|modified|deleted; please work from the latest state")

// Backend implements abstraction over local or remote storage backend.
// Item keys are assumed to be valid UTF8, which may be enforced by the
// various Backend implementations.
type Backend interface {
	// GetName returns the implementation driver name.
	GetName() string

	// Create creates item if it does not exist
	Create(ctx context.Context, i Item) (*Lease, error)

	// Update updates the value in the backend if the revision of the [Item] matches
	// the stored revision.
	Update(ctx context.Context, i Item) (*Lease, error)

	// Put puts value into backend (creates if it does not
	// exists, updates it otherwise)
	// Prefer using Create or Update for conditional operations.
	Put(ctx context.Context, i Item) (*Lease, error)

	// Get returns a single item or not found error
	Get(ctx context.Context, key Key) (*Item, error)

	// Items produces an iterator of backend items in the range, and order
	// described in the provided [ItemsParams].
	Items(ctx context.Context, params ItemsParams) iter.Seq2[Item, error]

	// Delete deletes item by key, returns NotFound error
	// if item does not exist
	Delete(ctx context.Context, key Key) error

	// KeepAlive keeps object from expiring, updates lease on the existing object,
	// expires contains the new expiry to set on the lease,
	// some backends may ignore expires based on the implementation
	// in case if the lease managed server side
	KeepAlive(ctx context.Context, lease Lease, expires time.Time) error

	// Close closes backend and all associated resources
	Close() error
}
