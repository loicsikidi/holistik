package backend

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// Separator is used as a separator between key parts.
	Separator = '/'
	// SeparatorString is string representation of Separator.
	SeparatorString = string(Separator)

	noEnd = "\x00"
)

// Key is the unique identifier for an [Item].
type Key struct {
	s          string
	components []string
	exactKey   bool
	noEnd      bool
}

// NewKey joins parts into path separated by [Separator],
// makes sure path always starts with [Separator].
func NewKey(components ...string) Key {
	if len(components) < 1 {
		return Key{}
	}
	k := Key{
		components: slices.Clone(components),
	}
	k.s = SeparatorString + strings.Join(k.components, SeparatorString)
	k.exactKey = k.s[len(k.s)-1] == Separator
	return k
}

func (k Key) IsZero() bool {
	return len(k.components) == 0 && k.s == ""
}

// AppendKey returns a new [Key] that joins p and k
// with the components of k followed by the components from p.
func (k Key) AppendKey(p Key) Key {
	if k.IsZero() {
		return p
	}

	newKey := Key{
		components: slices.Concat(k.components, p.components),
	}
	if strings.HasPrefix(k.s, SeparatorString) {
		newKey.s = SeparatorString + strings.Join(newKey.components, SeparatorString)
	} else {
		newKey.s = strings.Join(newKey.components, SeparatorString)
	}

	return newKey
}

// ExactKey appends a [Separator] to the key, if one does not already
// exist. This is to ensure range matching of a path will only
// math child paths and not other paths that have the resulting path
// as a prefix.
func (k Key) ExactKey() Key {
	if k.exactKey {
		return k
	}

	return ExactKey(k.components...)
}

// ExactKey is like [NewKey], except a [Separator] is appended to the
// result path of [Key]. This is to ensure range matching of a path will only
// math child paths and not other paths that have the resulting path
// as a prefix.
func ExactKey(components ...string) Key {
	k := Key{
		components: slices.Concat(components, []string{""}),
	}
	k.s = SeparatorString + strings.Join(k.components, SeparatorString)
	k.exactKey = true
	return k
}

// String returns the textual representation of the key with
// each component concatenated together via the [Separator].
func (k Key) String() string {
	if k.noEnd {
		return noEnd
	}

	return k.s
}

// TrimPrefix returns the key without the provided leading prefix string.
// If the key doesn't start with prefix, it is returned unchanged.
func (k Key) TrimPrefix(prefix Key) Key {
	key := strings.TrimPrefix(k.s, prefix.s)
	if key == "" {
		return Key{}
	}

	return KeyFromString(key)
}

// TrimPrefixAndSeparator removes the provided prefix from the key and then removes
// the leading [Separator] if present. This is useful for converting backend keys
// to API-relative keys.
//
// Example:
//
//	prefix := backend.NewKey("app", "data")
//	key := backend.NewKey("app", "data", "users", "123")
//	result := key.TrimPrefixAndSeparator(prefix)
//	// result.String() == "users/123"
func (k Key) TrimPrefixAndSeparator(prefix Key) string {
	trimmed := k.TrimPrefix(prefix).String()
	return strings.TrimPrefix(trimmed, SeparatorString)
}

// RangeEnd returns end of the range for given key.
func RangeEnd(key Key) Key {
	end := make([]byte, len(key.s))
	copy(end, key.s)
	for i := len(end) - 1; i >= 0; i-- {
		if end[i] < 0xff {
			end[i] = end[i] + 1
			end = end[:i+1]
			return KeyFromString(string(end))
		}
	}
	// next key does not exist (e.g., 0xffff);
	return Key{noEnd: true}
}

// KeyFromString creates a [Key] from a textual representation
// of the [Key]. No leading or trailing [Separator] are added
// like with [NewKey] or [ExactKey].
func KeyFromString(s string) Key {
	components := strings.Split(s, SeparatorString)
	if components[0] == "" && len(components) > 1 {
		components = components[1:]
	}

	return Key{
		components: components,
		s:          s,
		exactKey:   s == SeparatorString || (s != "" && s[len(s)-1] == Separator),
		noEnd:      s == noEnd,
	}
}

// Item is a key value item
type Item struct {
	// Key is a key of the key value item
	Key Key
	// Value is a value of the key value item
	Value []byte
	// Expires is an optional record expiry time
	Expires time.Time
	// Revision is the last known version of the object.
	Revision string
}

// Lease represents a lease on the item that can be used
// to extend item's TTL without updating its contents.
//
// Here is an example of renewing object TTL:
//
//	item.Expires = time.Now().Add(10 * time.Second)
//	lease, err := backend.Create(ctx, item)
//	expires := time.Now().Add(20 * time.Second)
//	err = backend.KeepAlive(ctx, lease, expires)
type Lease struct {
	// Key is the resource identifier.
	Key Key
	// Revision is the last known version of the object.
	Revision string
}

// ItemsParams are parameters that are provided to
// alter the iteration behavior.
type ItemsParams struct {
	// StartKey is the minimum key in the range yielded by the iteration. This key
	// will be included in the results if it exists.
	StartKey Key
	// EndKey is the maximum key in the range yielded by the iteration. This key
	// will be included in the results if it exists.
	EndKey Key
	// Descending makes the iteration yield items from the biggest to the smallest
	// key (i.e. from EndKey to StartKey). If unset, the iteration will proceed in the
	// usual ascending order (i.e. from StartKey to EndKey).
	Descending bool
	// Limit is an optional maximum number of items to retrieve during iteration.
	Limit int
}

// CreateRevision generates a new UUID to be used a revision.
//
// Note: this method should be replaced by backend implementations that provide
// their own mechanism for versioning resources.
func CreateRevision() string {
	return uuid.NewString()
}
