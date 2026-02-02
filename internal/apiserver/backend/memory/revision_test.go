package memory

import (
	"context"
	"testing"
	"time"

	"github.com/loicsikidi/holistik/internal/apiserver/backend"
	"github.com/loicsikidi/sentinel"
)

// TestStrictRevisionMatching validates that Update and KeepAlive strictly enforce
// revision matching as per the interface contract:
// "Update updates the value in the backend if the revision of the Item matches the stored revision."
//
// This test ensures that:
// 1. Update ALWAYS requires a matching revision
// 2. KeepAlive ALWAYS requires a matching revision
// 3. Empty revisions are rejected (they never match a valid stored revision)
// 4. Wrong revisions return backend.ErrIncorrectRevision
func TestStrictRevisionMatching(t *testing.T) {
	ctx := context.Background()

	t.Run("Update requires exact revision match", func(t *testing.T) {
		b, err := New(Config{})
		if err != nil {
			t.Fatal(err)
		}

		// Create an item with a revision
		lease, err := b.Create(ctx, backend.Item{
			Key:   backend.NewKey("test"),
			Value: []byte("original"),
		})
		if err != nil {
			t.Fatal(err)
		}

		// Test 1: Update with correct revision should succeed
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("updated"),
			Revision: lease.Revision,
		})
		if err != nil {
			t.Fatalf("update with correct revision should succeed: %v", err)
		}

		// Get the new revision after update
		item, err := b.Get(ctx, backend.NewKey("test"))
		if err != nil {
			t.Fatal(err)
		}

		// Test 2: Update with empty revision should fail
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("should fail"),
			Revision: "", // Empty revision
		})
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for empty revision, got: %v", err)
		}
		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should be a CompareFailed type: %v", err)
		}

		// Test 3: Update with wrong revision should fail
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("should fail"),
			Revision: "wrong-revision-12345",
		})
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for wrong revision, got: %v", err)
		}
		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should be a CompareFailed type: %v", err)
		}

		// Test 4: Update with old revision should fail
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("should fail"),
			Revision: lease.Revision, // Old revision from first create
		})
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for old revision, got: %v", err)
		}

		// Verify the item was not modified by failed updates
		finalItem, err := b.Get(ctx, backend.NewKey("test"))
		if err != nil {
			t.Fatal(err)
		}
		if string(finalItem.Value) != "updated" {
			t.Errorf("value should still be 'updated', got: %q", string(finalItem.Value))
		}
		if finalItem.Revision != item.Revision {
			t.Errorf("revision should not have changed after failed updates")
		}
	})

	t.Run("KeepAlive requires exact revision match", func(t *testing.T) {
		b, err := New(Config{})
		if err != nil {
			t.Fatal(err)
		}

		// Create an item with a revision
		lease, err := b.Create(ctx, backend.Item{
			Key:   backend.NewKey("test"),
			Value: []byte("data"),
		})
		if err != nil {
			t.Fatal(err)
		}

		newExpiry := time.Now().Add(2 * time.Hour)

		// Test 1: KeepAlive with correct revision should succeed
		err = b.KeepAlive(ctx, *lease, newExpiry)
		if err != nil {
			t.Fatalf("keepalive with correct revision should succeed: %v", err)
		}

		// Test 2: KeepAlive with empty revision should fail
		err = b.KeepAlive(ctx, backend.Lease{
			Key:      backend.NewKey("test"),
			Revision: "", // Empty revision
		}, newExpiry)
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for empty revision, got: %v", err)
		}
		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should be a CompareFailed type: %v", err)
		}

		// Test 3: KeepAlive with wrong revision should fail
		err = b.KeepAlive(ctx, backend.Lease{
			Key:      backend.NewKey("test"),
			Revision: "wrong-revision-67890",
		}, newExpiry)
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for wrong revision, got: %v", err)
		}
		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should be a CompareFailed type: %v", err)
		}

		// Update the item to change its revision
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("modified"),
			Revision: lease.Revision,
		})
		if err != nil {
			t.Fatal(err)
		}

		// Test 4: KeepAlive with old revision (from before Update) should fail
		err = b.KeepAlive(ctx, *lease, newExpiry)
		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected backend.ErrIncorrectRevision for old revision after update, got: %v", err)
		}
	})

	t.Run("Revision mismatch returns correct error", func(t *testing.T) {
		b, err := New(Config{})
		if err != nil {
			t.Fatal(err)
		}

		lease, err := b.Create(ctx, backend.Item{
			Key:   backend.NewKey("test"),
			Value: []byte("data"),
		})
		if err != nil {
			t.Fatal(err)
		}

		// Test that the error is exactly backend.ErrIncorrectRevision
		_, err = b.Update(ctx, backend.Item{
			Key:      backend.NewKey("test"),
			Value:    []byte("new"),
			Revision: "wrong",
		})

		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected error to be backend.ErrIncorrectRevision, got: %v", err)
		}

		// Test that it satisfies sentinel.IsCompareFailed
		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should satisfy sentinel.IsCompareFailed")
		}

		// Same test for KeepAlive
		err = b.KeepAlive(ctx, backend.Lease{
			Key:      lease.Key,
			Revision: "wrong",
		}, time.Now().Add(1*time.Hour))

		if err != backend.ErrIncorrectRevision {
			t.Errorf("expected error to be backend.ErrIncorrectRevision, got: %v", err)
		}

		if !sentinel.IsCompareFailed(err) {
			t.Errorf("error should satisfy sentinel.IsCompareFailed")
		}
	})
}
