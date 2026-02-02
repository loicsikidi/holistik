package validation

import (
	"regexp"
	"slices"
	"strings"

	"github.com/loicsikidi/holistik/internal/apiserver/api/resources/types"
	"github.com/loicsikidi/holistik/internal/apiserver/services/internal/generic"
	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"
	"github.com/loicsikidi/sentinel"
)

// Validator validates a resource and returns an error if validation fails.
// Validators can be composed using [Chain] to build complex validation logic.
//
// The type parameter T can be any type that satisfies resources.Resource.
// For validators that need access to GetSpec(), use a concrete type that has this method.
type Validator[T generic.Resource] func(T) error

// Chain combines multiple validators into a single validator that runs them in sequence.
// Validation stops at the first error encountered.
//
// Example:
//
//	validator := validation.Chain(
//	    validation.CheckAndSetDefaultsValidator[*MyResource](),
//	    headerValidator,
//	    specValidator,
//	)
func Chain[T generic.Resource](validators ...Validator[T]) Validator[T] {
	return func(resource T) error {
		for _, v := range validators {
			if err := v(resource); err != nil {
				return err
			}
		}
		return nil
	}
}

// CheckAndSetDefaultsValidator wraps the CheckAndSetDefaults pattern as a validator.
// This allows existing CheckAndSetDefaults implementations to be used in validator chains.
//
// If the resource implements CheckAndSetDefaults() error, it will be called.
// If the resource does not implement the method, this is a no-op.
func CheckAndSetDefaultsValidator[T generic.Resource]() Validator[T] {
	return func(resource T) error {
		if r, ok := any(resource).(interface{ CheckAndSetDefaults() error }); ok {
			return sentinel.Wrap(r.CheckAndSetDefaults())
		}
		return nil
	}
}

// ResourceWithSpec is a [generic.Resource] with the constraint that it has to implement GetSpec method.
//
// This way, we ensure that the given callback has access to the spec field of the resource.
type ResourceWithSpec[T any] interface {
	generic.Resource
	GetSpec() *T
}

// SpecValidator creates a validator for resource spec validation.
// The provided function should validate the spec fields of the resource.
//
// Example:
//
//	specVal := validation.SpecValidator[*MyResource](
//	    func(r *MyResource) error {
//	        if r.GetSpec() == nil {
//	            return sentinel.BadParameter("spec is required")
//	        }
//	        return nil
//	    },
//	)
func SpecValidator[U any, T ResourceWithSpec[U]](fn func(T) error) Validator[T] {
	return fn
}

// CommonValidator creates a validator for common fields.
//
// The validator checks the resource against the provided configuration.
// It validates Kind, Version, SubKind, and Name according to the config rules.
// It also checks that the Spec field is non-nil.
//
// Example:
//
//	headerVal, err := validation.CommonValidator[*MyResource](
//	    validation.CommonValidatorConfig{
//	        ExpectedKind:      "my_resource",
//	        AllowedVersions:   []string{"v1"},
//	        NamePattern:       `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
//	    },
//	)
//	if err != nil {
//	    // Handle config error
//	}
func CommonValidator[T generic.Resource](cfg CommonValidatorConfig) (Validator[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	return func(resource T) error {
		// Use type assertion to check if resource implements the headerResource interface
		header, ok := any(resource).(commonResourceInfo)
		if !ok {
			return sentinel.BadParameter("resource does not implement required methods (GetKind, GetVersion, GetSubKind)")
		}

		if err := validateKind(header, cfg.ExpectedKind); err != nil {
			return err
		}

		if err := validateVersion(header, cfg.AllowedVersions); err != nil {
			return err
		}

		if err := validateSubKind(header, cfg); err != nil {
			return err
		}

		metadata := resource.GetMetadata()
		if metadata == nil {
			return sentinel.BadParameter("resource metadata is nil")
		}

		if err := validateName(metadata, cfg); err != nil {
			return err
		}

		// Use type assertion to check if spec is nil
		specGetter, ok := any(resource).(interface{ GetSpec() any })
		if ok && specGetter.GetSpec() == nil {
			return sentinel.BadParameter("resource spec is nil")
		}

		return nil
	}, nil
}

// CommonValidatorConfig configures common header validation.
type CommonValidatorConfig struct {
	// ExpectedKind is the required Kind value.
	//
	// Required.
	ExpectedKind string

	// AllowedVersions specifies valid Version values.
	//
	// Optional. Defaults to ["v1"] if not set.
	AllowedVersions []string

	// AllowedSubKinds specifies valid SubKind values.
	//
	// Optional. If empty, SubKind validation is skipped (any value is allowed, including empty).
	AllowedSubKinds []string

	// NamePattern is a regex pattern for name validation.
	// Optional. If empty, skips this check.
	NamePattern string

	// compiled regex, set during CheckAndSetDefaults
	nameRegex *regexp.Regexp
}

// CheckAndSetDefaults validates and compiles configuration.
func (c *CommonValidatorConfig) CheckAndSetDefaults() error {
	if c.ExpectedKind == "" {
		return sentinel.BadParameter("expected kind is required")
	}

	if len(c.AllowedVersions) == 0 {
		c.AllowedVersions = []string{types.V1}
	}

	if c.NamePattern != "" {
		re, err := regexp.Compile(c.NamePattern)
		if err != nil {
			return sentinel.BadParameter("invalid name pattern: %v", err)
		}
		c.nameRegex = re
	}

	return nil
}

// validateKind validates the Kind field.
func validateKind(header commonResourceInfo, expectedKind string) error {
	if expectedKind != "" && header.GetKind() != expectedKind {
		return sentinel.BadParameter("expected kind %q, got %q", expectedKind, header.GetKind())
	}
	return nil
}

// validateVersion validates the Version field.
func validateVersion(header commonResourceInfo, allowedVersions []string) error {
	version := header.GetVersion()
	if !slices.Contains(allowedVersions, version) {
		return sentinel.BadParameter("invalid version %q, allowed values: %s",
			version, strings.Join(allowedVersions, ", "))
	}

	return nil
}

// validateSubKind validates the SubKind field.
func validateSubKind(header commonResourceInfo, cfg CommonValidatorConfig) error {
	// Skip validation if no allowed subkinds are configured
	if len(cfg.AllowedSubKinds) == 0 {
		return nil
	}

	subKind := header.GetSubKind()
	if !slices.Contains(cfg.AllowedSubKinds, subKind) {
		return sentinel.BadParameter("invalid subkind %q, allowed values: %s",
			subKind, strings.Join(cfg.AllowedSubKinds, ", "))
	}

	return nil
}

// validateName validates the Name field in metadata.
func validateName(metadata *headerv1.Metadata, cfg CommonValidatorConfig) error {
	name := metadata.GetName()

	if cfg.nameRegex != nil && !cfg.nameRegex.MatchString(name) {
		return sentinel.BadParameter("name %q does not match required pattern %q",
			name, cfg.NamePattern)
	}

	return nil
}

// commonResourceInfo is an interface that provides common field accessors.
type commonResourceInfo interface {
	GetKind() string
	GetVersion() string
	GetSubKind() string
}
