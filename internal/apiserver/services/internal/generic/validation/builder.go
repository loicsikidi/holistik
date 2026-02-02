package validation

import (
	"github.com/loicsikidi/sentinel"
)

// ValidatorConfig configures a complete resource validator.
type ValidatorConfig[U any, T ResourceWithSpec[U]] struct {
	// Common contains the configuration for common fields validation (Kind, Version, Name, etc).
	//
	// Required.
	Common CommonValidatorConfig

	// SpecValidatorFunc is a required callback to validate the Spec field.
	//
	// Required.
	SpecValidatorFunc Validator[T]
}

// CheckAndSetDefaults validates the ValidatorConfig.
func (c *ValidatorConfig[U, T]) CheckAndSetDefaults() error {
	if c.SpecValidatorFunc == nil {
		return sentinel.BadParameter("SpecValidator is required")
	}

	if err := c.Common.CheckAndSetDefaults(); err != nil {
		return sentinel.Wrap(err)
	}

	return nil
}

// NewValidator creates a complete validator from configuration.
// It chains together CheckAndSetDefaults, Common, and Spec validators.
//
// Example:
//
//	validator, err := validation.NewValidator[MySpec, *MyResource](
//	    validation.ValidatorConfig[MySpec, *MyResource]{
//	        Common: validation.CommonValidatorConfig{
//	            ExpectedKind:    "my_resource",
//	            AllowedVersions: []string{"v1"},
//	            NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
//	        },
//	        SpecValidator: func(r *MyResource) error {
//	            if r.GetSpec().GetField() == "" {
//	                return sentinel.BadParameter("spec.field is required")
//	            }
//	            return nil
//	        },
//	    },
//	)
//	if err != nil {
//	    return err
//	}
func NewValidator[U any, T ResourceWithSpec[U]](cfg ValidatorConfig[U, T]) (Validator[T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	// ignoring error as it should be validated in CheckAndSetDefaults
	commonValidator, _ := CommonValidator[T](cfg.Common)

	// We respect the following order:
	// 1. CheckAndSetDefaults
	// 2. Common fields validation
	// 3. Spec validation
	validators := []Validator[T]{
		CheckAndSetDefaultsValidator[T](),
		commonValidator,
		cfg.SpecValidatorFunc,
	}

	return Chain(validators...), nil
}
