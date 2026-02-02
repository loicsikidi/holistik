// Package validation provides a composable validation system for resources.
//
// # Overview
//
// This package implements a type-safe, composable validation system for resources.
// It centralizes common metadata validation (kind, version, subkind, name) while
// enabling resource-specific validation through composition.
//
// # Quick Start
//
// For most use cases, use [NewValidator] which provides a simple, declarative API
// that handles the complete validation flow:
//
//	validator, err := validation.NewValidator[MySpec, *MyResource](
//	    validation.ValidatorConfig[MySpec, *MyResource]{
//	        Common: validation.CommonValidatorConfig{
//	            ExpectedKind:    "my_resource",
//	            AllowedVersions: []string{"v1"},
//	            NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
//	        },
//	        SpecValidatorFunc: func(r *MyResource) error {
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
//
// [NewValidator] automatically chains together:
//  1. CheckAndSetDefaults() - Sets defaults and validates invariants
//  2. Common fields validation - Validates Kind, Version, SubKind, Name
//  3. Spec validation - Your custom resource-specific validation
//
// # Core Concepts
//
// A [Validator] is a function that validates a resource and returns an error:
//
//	type Validator[T generic.Resource] func(T) error
//
// Validators can be combined using [Chain] to build complex validation logic.
//
// # Validation Flow
//
// The validation flow follows this pattern:
//
//	CheckAndSetDefaults → Common Validation → Spec Validation
//	        ↓                    ↓                   ↓
//	   (defaults)         (kind, version,     (resource-specific
//	                       subkind, name)          rules)
//
// # Advanced Usage
//
// For advanced scenarios requiring custom validation chains, you can use the
// lower-level building blocks:
//
//	// Build validators separately
//	headerVal, err := validation.CommonValidator[*MyResource](
//	    validation.CommonValidatorConfig{
//	        ExpectedKind:    "my_resource",
//	        AllowedVersions: []string{"v1"},
//	        NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
//	    },
//	)
//	if err != nil {
//	    return err
//	}
//
//	specVal := validation.SpecValidator(
//	    func(r *MyResource) error {
//	        if r.GetSpec() == nil {
//	            return sentinel.BadParameter("spec is required")
//	        }
//	        return nil
//	    },
//	)
//
//	// Manually chain them together
//	validator := validation.Chain(
//	    validation.CheckAndSetDefaultsValidator[*MyResource](),
//	    headerVal,
//	    specVal,
//	    customValidator, // Add custom validators
//	)
//
// # Integration with Services
//
// The validator integrates seamlessly with [generic.ServiceConfig]:
//
//	validator, err := validation.NewValidator[MySpec, *MyResource](cfg)
//	if err != nil {
//	    return err
//	}
//
//	svc := &generic.ServiceConfig[*MyResource]{
//	    Backend:       backend,
//	    ResourceKind:  "my_resource",
//	    ValidateFunc:  validator,
//	    // ... other config
//	}
package validation
