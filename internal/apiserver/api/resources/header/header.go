package header

import (
	"maps"
	"time"

	"github.com/loicsikidi/sentinel"
)

// ResourceHeader is a common header for any resource.
type ResourceHeader struct {
	// Kind is a resource kind.
	Kind string `json:"kind,omitempty"`
	// SubKind is a resource sub kind.
	SubKind string `json:"sub_kind,omitempty"`
	// Version is the resource version.
	Version string `json:"version,omitempty"`
	// Metadata is metadata for the resource.
	Metadata Metadata `json:"metadata,omitempty"`
}

func (rh *ResourceHeader) Clone() *ResourceHeader {
	if rh == nil {
		return nil
	}

	return &ResourceHeader{
		Kind:     rh.Kind,
		SubKind:  rh.SubKind,
		Version:  rh.Version,
		Metadata: *rh.Metadata.Clone(),
	}
}

// GetKind returns the resource kind.
func (rh *ResourceHeader) GetKind() string {
	return rh.Kind
}

// SetKind sets the resource kind.
func (rh *ResourceHeader) SetKind(kind string) {
	rh.Kind = kind
}

// GetSubKind returns the resource subkind.
func (rh *ResourceHeader) GetSubKind() string {
	return rh.SubKind
}

// SetSubKind sets the resource subkind.
func (rh *ResourceHeader) SetSubKind(s string) {
	rh.SubKind = s
}

// GetVersion returns the resource version.
func (rh *ResourceHeader) GetVersion() string {
	return rh.Version
}

// SetVersion sets the resource version.
func (rh *ResourceHeader) SetVersion(version string) {
	rh.Version = version
}

// GetRevision returns the revision.
func (rh *ResourceHeader) GetRevision() string {
	return rh.Metadata.GetRevision()
}

// SetRevision sets the revision.
func (rh *ResourceHeader) SetRevision(rev string) {
	rh.Metadata.SetRevision(rev)
}

// GetName returns the name of the resource.
func (rh *ResourceHeader) GetName() string {
	return rh.Metadata.GetName()
}

// SetName sets the name of the resource.
func (rh *ResourceHeader) SetName(v string) {
	rh.Metadata.SetName(v)
}

// Expiry returns the resource expiry setting.
func (rh *ResourceHeader) Expiry() time.Time {
	return rh.Metadata.Expiry()
}

// SetExpiry sets the resource expiry.
func (rh *ResourceHeader) SetExpiry(t time.Time) {
	rh.Metadata.SetExpiry(t)
}

// GetLabel retrieves the label with the provided key.
//
// Note: this method return a tuple in order to distinguish between a missing label and an empty label value.
func (rh *ResourceHeader) GetLabel(key string) (string, bool) {
	return rh.Metadata.GetLabel(key)
}

// GetLabels returns all labels from the resource.
func (rh *ResourceHeader) GetLabels() map[string]string {
	return rh.Metadata.GetLabels()
}

// SetLabels sets the static labels for the resource.
func (rh *ResourceHeader) SetLabels(labels map[string]string) {
	rh.Metadata.SetLabels(labels)
}

// GetMetadata returns object metadata.
func (rh *ResourceHeader) GetMetadata() Metadata {
	return rh.Metadata
}

// CheckAndSetDefaults will verify that the resource header is valid. This will additionally
// verify that the metadata is valid.
func (rh *ResourceHeader) CheckAndSetDefaults() error {
	if rh.Kind == "" {
		return sentinel.BadParameter("missing parameter Kind")
	}
	if rh.Version == "" {
		return sentinel.BadParameter("missing parameter Version")
	}
	return sentinel.Wrap(rh.Metadata.CheckAndSetDefaults())
}

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string `json:"name" yaml:"name"`
	// Description is object description
	Description string `json:"description,omitempty"`
	// Labels is a set of labels
	Labels map[string]string `json:"labels,omitempty"`
	// Expires is a global expiry time header
	Expires time.Time `json:"expires"`
	// Revision is an opaque identifier which tracks the versions of a resource
	// over time.
	// This field is managed by the system.
	Revision string `json:"revision,omitempty"`
}

func (m *Metadata) Clone() *Metadata {
	if m == nil {
		return nil
	}

	return &Metadata{
		Name:        m.Name,
		Description: m.Description,
		Labels:      maps.Clone(m.Labels),
		Expires:     m.Expires,
		Revision:    m.Revision,
	}
}

// CheckAndSetDefaults verifies that the metadata object is valid.
func (m *Metadata) CheckAndSetDefaults() error {
	if m.Name == "" {
		return sentinel.BadParameter("missing parameter Name")
	}

	// the system relies on UTC for expiry times
	if !m.Expires.IsZero() {
		m.Expires = m.Expires.UTC()
	}

	return nil
}

// GetRevision returns the revision
func (m *Metadata) GetRevision() string {
	return m.Revision
}

// SetRevision sets the revision
func (m *Metadata) SetRevision(rev string) {
	m.Revision = rev
}

// GetName returns the name of the resource.
func (m *Metadata) GetName() string {
	return m.Name
}

// SetName sets the name of the resource.
func (m *Metadata) SetName(name string) {
	m.Name = name
}

// SetExpiry sets the expiry time for the object.
func (m *Metadata) SetExpiry(expires time.Time) {
	m.Expires = expires
}

// Expiry returns the object expiry setting.
func (m *Metadata) Expiry() time.Time {
	return m.Expires
}

// GetLabel retrieves the label with the provided key.
//
// Note: this method return a tuple in order to distinguish between a missing label and an empty label value.
func (m *Metadata) GetLabel(key string) (label string, ok bool) {
	label, ok = m.Labels[key]
	return label, ok
}

// GetLabels returns all labels from the resource.
func (m *Metadata) GetLabels() map[string]string {
	return m.Labels
}

// SetLabels sets labels for the metadata.
func (m *Metadata) SetLabels(labels map[string]string) {
	m.Labels = labels
}
