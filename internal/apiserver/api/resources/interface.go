/*
 * This file define core-interface in order to apply design recommandation defined in RFD 153: Resource Guideline
 * Source: https://github.com/gravitational/teleport/blob/master/rfd/0153-resource-guidelines.md
 *
 * Essentially our implementation will mainly rely on protobuf representation of each resource. We can do
 * that because our repository is build from stratch. In addition, the simplicity of BaseResource interface
 * allow us to fallback to an internal Go struct representation if needed.
 */
package resources

import (
	"time"

	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"
	"github.com/loicsikidi/sentinel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Resource is the smallest interface that defines a resource in accordance with RFD 153: Resource Guideline.
//
// This interface must be implemented in order to benefit from Generic GRPC Service Factory.
// (see internal/apiserver/services/internal/generic/generic.go for more details)
//
// The latter centralizes at one place service RPCs as defined in "API" section from RFD 153.
type Resource interface {
	// GetKind returns the kind of the resource.
	//
	// Required.
	GetKind() string

	// GetSubKind returns the subkind of the resource.
	//
	// Optional.
	GetSubKind() string

	// GetVersion returns the version of the resource.
	//
	// Required.
	GetVersion() string

	// GetMetadata returns the generic resource metadata.
	//
	// Required.
	GetMetadata() *headerv1.Metadata
}

// GetRevision returns an identifier of the resource over time.
//
// Note: this method can be used by a protobuf representation of a resource or an internal Go struct.
func GetRevision(v any) (string, error) {
	switch r := v.(type) {
	case Resource:
		return r.GetMetadata().GetRevision(), nil
	}
	return "", sentinel.BadParameter("unable to determine revision from resource of type %T", v)
}

// SetRevision sets an identifier of the resource over time.
//
// Note: this method can be used by a protobuf representation of a resource or an internal Go struct.
func SetRevision(resource any, revision string) error {
	switch r := resource.(type) {
	case Resource:
		r.GetMetadata().Revision = revision
		return nil
	}
	return sentinel.BadParameter("unable to set revision on resource of type %T", resource)
}

// GetExpiry returns the expiration time of the resource.
//
// Note: this method can be used by a protobuf representation of a resource or an internal Go struct.
func GetExpiry(resource any) (time.Time, error) {
	switch r := resource.(type) {
	case Resource:
		// On creation/update a resource without expiration will have an expiration timestamp set to nil.
		// In this case, we treat the expiration as zero time (i.e. time.Time{}).
		exp := r.GetMetadata().GetExpires()
		if exp == nil {
			return time.Time{}, nil
		}
		return exp.AsTime(), nil
	}
	return time.Time{}, sentinel.BadParameter("unable to determine expiry from resource of type %T", resource)
}

// SetExpiry sets the expiration time of the resource.
//
// Note: this method can be used by a protobuf representation of a resource or an internal Go struct.
func SetExpiry(resource any, expiry time.Time) error {
	switch r := resource.(type) {
	case Resource:
		if !expiry.IsZero() {
			expiryUTC := expiry.UTC()
			r.GetMetadata().Expires = timestamppb.New(expiryUTC)
		}
		return nil
	}
	return sentinel.BadParameter("unable to set expiry on resource of type %T", resource)
}

// SetLabels sets the labels of the resource.
//
// Note: this method can be used by a protobuf representation of a resource or an internal Go struct.
func SetLabels(resource any, labels map[string]string) error {
	switch r := resource.(type) {
	case Resource:
		r.GetMetadata().Labels = labels
		return nil
	}
	return sentinel.BadParameter("unable to set labels on resource of type %T", resource)
}
