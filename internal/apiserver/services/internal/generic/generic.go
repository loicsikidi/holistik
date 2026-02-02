package generic

import (
	"context"
	"iter"
	"log/slog"

	"github.com/loicsikidi/holistik/internal/apiserver/api/resources"
	"github.com/loicsikidi/holistik/internal/apiserver/backend"
	"github.com/loicsikidi/holistik/internal/apiserver/services"
	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"
	"github.com/loicsikidi/sentinel"
)

const (
	// DefaultPageSize is the default chunk size for paginated endpoints.
	DefaultPageSize = 500
)

// MarshalFunc is a type signature for a marshaling function, which converts from T to []byte.
type MarshalFunc[T any] func(T, ...MarshalConfig) ([]byte, error)

// UnmarshalFunc is a type signature for an unmarshalling function, which converts from []byte to T.
type UnmarshalFunc[T any] func([]byte, ...MarshalConfig) (T, error)

// ServiceConfig is the configuration for the service configuration.
type ServiceConfig[T Resource] struct {
	// Backend used to persist the resource.
	Backend backend.Backend
	// ResourceKind is the friendly name of the resource.
	ResourceKind string
	// PageLimit
	PageLimit int
	// BackendPrefix used when constructing the [backend.Item.Key].
	BackendPrefix backend.Key
	// MarshlFunc converts the resource to bytes for persistence.
	MarshalFunc MarshalFunc[T]
	// UnmarshalFunc converts the bytes read from the backend to the resource.
	UnmarshalFunc UnmarshalFunc[T]
	// ValidateFunc optionally validates the resource prior to persisting it. Any errors
	// returned from the validation function will prevent writes to the backend.
	ValidateFunc func(T) error
	// NameKeyFunc optionally allows resources to have a custom key suffix, by
	// transforming the name of the resource or the input given to methods that
	// take a resource name. If unset, the name is used without changes.
	NameKeyFunc func(name string) string
}

func (c *ServiceConfig[T]) CheckAndSetDefaults() error {
	if c.Backend == nil {
		return sentinel.BadParameter("backend is missing")
	}
	if c.ResourceKind == "" {
		return sentinel.BadParameter("resource kind is missing")
	}
	// We should allow page limit to be 0 for services that don't use pagination.
	if c.PageLimit <= 0 {
		c.PageLimit = DefaultPageSize
	}
	if c.BackendPrefix.IsZero() {
		return sentinel.BadParameter("backend prefix is missing")
	}
	if c.MarshalFunc == nil {
		return sentinel.BadParameter("marshal func is missing")
	}
	if c.UnmarshalFunc == nil {
		return sentinel.BadParameter("unmarshal func is missing")
	}

	if c.ValidateFunc == nil {
		// allow all by default
		c.ValidateFunc = func(T) error { return nil }
	}

	return nil
}

// Resource is the smallest interface to represent a resource.
//
// Note: this representation will ease unit testing and mocking.
type Resource interface {
	GetMetadata() *headerv1.Metadata
}

func getResourceName(resource Resource) string {
	if metadata := resource.GetMetadata(); metadata != nil {
		return metadata.GetName()
	}
	return ""
}

// Service is a generic service for interacting with resources in the backend.
type Service[T Resource] struct {
	backend       backend.Backend
	resourceKind  string
	pageLimit     int
	backendPrefix backend.Key
	marshalFunc   MarshalFunc[T]
	unmarshalFunc UnmarshalFunc[T]
	validateFunc  func(T) error
	// runWhileLockedRetryInterval time.Duration
	nameKeyFunc func(name string) string
}

// NewService will return a new generic service with the given config. This will
// panic if the configuration is invalid.
func NewService[T Resource](cfg *ServiceConfig[T]) (*Service[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, sentinel.Wrap(err)
	}

	return &Service[T]{
		backend:       cfg.Backend,
		resourceKind:  cfg.ResourceKind,
		pageLimit:     cfg.PageLimit,
		backendPrefix: cfg.BackendPrefix,
		marshalFunc:   cfg.MarshalFunc,
		unmarshalFunc: cfg.UnmarshalFunc,
		validateFunc:  cfg.ValidateFunc,
		nameKeyFunc:   cfg.NameKeyFunc,
	}, nil
}

// WithPrefix will return a service with the given parts appended to the backend prefix.
func (s *Service[T]) WithPrefix(parts ...string) *Service[T] {
	if len(parts) == 0 {
		return s
	}
	s2 := *s
	s2.backendPrefix = s2.backendPrefix.AppendKey(backend.NewKey(parts...))
	return &s2
}

// MakeKey will make a key for the service given a name.
func (s *Service[T]) MakeKey(k backend.Key) backend.Key {
	return s.backendPrefix.AppendKey(k)
}

func (s *Service[T]) nameKey(name string) string {
	if s.nameKeyFunc != nil {
		return s.nameKeyFunc(name)
	}
	return name
}

// MakeBackendItem will check and make the backend item.
func (s *Service[T]) MakeBackendItem(resource T) (backend.Item, error) {
	if err := services.CheckAndSetDefaults(resource); err != nil {
		return backend.Item{}, sentinel.Wrap(err)
	}

	rev, err := resources.GetRevision(resource)
	if err != nil {
		return backend.Item{}, sentinel.Wrap(err)
	}

	value, err := s.marshalFunc(resource)
	if err != nil {
		return backend.Item{}, sentinel.Wrap(err)
	}

	item := backend.Item{
		Key:      s.MakeKey(backend.NewKey(s.nameKey(getResourceName(resource)))),
		Value:    value,
		Revision: rev,
	}

	item.Expires, err = resources.GetExpiry(resource)
	if err != nil {
		return backend.Item{}, sentinel.Wrap(err)
	}

	return item, nil
}

// CreateResource creates a new resource.
func (s *Service[T]) CreateResource(ctx context.Context, resource T) (T, error) {
	var t T
	if err := s.validateFunc(resource); err != nil {
		return t, sentinel.Wrap(err)
	}

	item, err := s.MakeBackendItem(resource)
	if err != nil {
		return t, sentinel.Wrap(err)
	}

	lease, err := s.backend.Create(ctx, item)
	if sentinel.IsAlreadyExists(err) {
		return t, sentinel.AlreadyExists("%s %q already exists", s.resourceKind, getResourceName(resource))
	}
	if err != nil {
		return t, sentinel.Wrap(err)
	}

	if err := resources.SetRevision(resource, lease.Revision); err != nil {
		return t, sentinel.Wrap(err)
	}

	return resource, nil
}

// UpdateResource updates an existing resource.
func (s *Service[T]) UpdateResource(ctx context.Context, resource T) (T, error) {
	var t T
	if err := s.validateFunc(resource); err != nil {
		return t, sentinel.Wrap(err)
	}

	item, err := s.MakeBackendItem(resource)
	if err != nil {
		return t, sentinel.Wrap(err)
	}

	lease, err := s.backend.Update(ctx, item)
	if err != nil {
		if sentinel.IsNotFound(err) {
			return t, sentinel.NotFound("%s %q doesn't exist", s.resourceKind, getResourceName(resource))
		}
		if sentinel.IsCompareFailed(err) {
			return t, sentinel.CompareFailed("%s %q has been modified, please refresh and try again", s.resourceKind, getResourceName(resource))
		}
		return t, sentinel.Wrap(err)
	}

	if err := resources.SetRevision(resource, lease.Revision); err != nil {
		return t, sentinel.Wrap(err)
	}

	return resource, nil
}

// GetResource returns the specified resource.
func (s *Service[T]) GetResource(ctx context.Context, name string) (resource T, err error) {
	item, err := s.backend.Get(ctx, s.MakeKey(backend.NewKey(s.nameKey(name))))
	if err != nil {
		if sentinel.IsNotFound(err) {
			return resource, sentinel.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return resource, sentinel.Wrap(err)
	}
	cfg := MarshalConfig{
		Expires:  item.Expires,
		Revision: item.Revision,
	}
	resource, err = s.unmarshalFunc(item.Value, cfg)
	return resource, sentinel.Wrap(err)
}

// CountResources will return a count of all resources in the prefix range.
func (s *Service[T]) CountResources(ctx context.Context) (uint, error) {
	rangeStart := s.backendPrefix.ExactKey()
	rangeEnd := backend.RangeEnd(rangeStart)

	count := uint(0)
	for _, err := range s.backend.Items(ctx, backend.ItemsParams{
		StartKey: rangeStart,
		EndKey:   rangeEnd,
	}) {
		if err != nil {
			return 0, sentinel.Wrap(err)
		}

		count++
	}

	return count, nil
}

// DeleteResource removes the specified resource.
func (s *Service[T]) DeleteResource(ctx context.Context, name string) error {
	err := s.backend.Delete(ctx, s.MakeKey(backend.NewKey(s.nameKey(name))))
	if err != nil {
		if sentinel.IsNotFound(err) {
			return sentinel.NotFound("%s %q doesn't exist", s.resourceKind, name)
		}
		return sentinel.Wrap(err)
	}
	return nil
}

// Resources returns a stream of resources within the range [startKey, endKey].
// If both keys are empty, then the entire range is returned.
func (s *Service[T]) Resources(ctx context.Context, startKey, endKey string) iter.Seq2[T, error] {
	params := backend.ItemsParams{
		StartKey: s.backendPrefix.AppendKey(backend.KeyFromString(startKey)),
	}
	if endKey == "" {
		params.EndKey = backend.RangeEnd(s.backendPrefix.ExactKey())
	} else {
		params.EndKey = s.backendPrefix.AppendKey(backend.KeyFromString(endKey))
	}
	return func(yield func(T, error) bool) {
		for item, err := range s.backend.Items(ctx, params) {
			if err != nil {
				var t T
				yield(t, sentinel.Wrap(err))
				return
			}

			cfg := MarshalConfig{
				Expires:  item.Expires,
				Revision: item.Revision,
			}
			resource, err := s.unmarshalFunc(item.Value, cfg)
			if err != nil {
				// unmarshal errors are logged and skipped
				slog.WarnContext(ctx, "skipping resource due to unmarshal error", "error", err, "key", item.Key.String())
				return
			}

			if !yield(resource, nil) {
				return
			}
		}
	}
}

// ListResources returns a paginated list of resources.
func (s *Service[T]) ListResources(ctx context.Context, pageSize int, pageToken string) (result []T, nextToken string, err error) {
	if pageSize <= 0 || pageSize > s.pageLimit {
		pageSize = s.pageLimit
	}

	var out []T
	for item, err := range s.backend.Items(ctx, backend.ItemsParams{
		StartKey: s.backendPrefix.AppendKey(backend.KeyFromString(pageToken)),
		EndKey:   backend.RangeEnd(s.backendPrefix.ExactKey()),
		Limit:    pageSize + 1,
	}) {
		if err != nil {
			return nil, "", sentinel.Wrap(err)
		}
		cfg := MarshalConfig{
			Expires:  item.Expires,
			Revision: item.Revision,
		}
		resource, err := s.unmarshalFunc(item.Value, cfg)
		// We make sure to respect the following rule from RFD 153:
		// "A listing operation should not abort entirely if a single item cannot be converted from a backend.Item,
		// it should instead be logged, and the rest of the page should be processed."
		if err != nil {
			slog.WarnContext(ctx, "skipping resource due to unmarshal error", slog.Any("error", err), slog.String("key", item.Key.String()))
			continue
		}

		if len(out) == pageSize {
			nextKey := item.Key.TrimPrefixAndSeparator(s.backendPrefix)
			return out, nextKey, nil
		}

		out = append(out, resource)
	}

	return out, "", nil
}
