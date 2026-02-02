package validation_test

import (
	"fmt"

	"github.com/loicsikidi/holistik/internal/apiserver/services/internal/generic/validation"
	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"
	testv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/test/v1"

	"github.com/loicsikidi/sentinel"
)

// Example_newValidator demonstrates using NewValidator for simplified setup.
func Example_newValidator() {
	// Create a complete validator using NewValidator
	validator, err := validation.NewValidator(
		validation.ValidatorConfig[testv1.TestResourceSpec, *testv1.TestResource]{
			Common: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1"},
				NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
			},
			SpecValidatorFunc: func(r *testv1.TestResource) error {
				if r.GetSpec() == nil {
					return sentinel.BadParameter("spec is required")
				}
				return nil
			},
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create validator: %v", err))
	}

	// Validate a resource
	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "my-test-resource"},
		Spec:     &testv1.TestResourceSpec{},
	}

	if err := validator(resource); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		return
	}

	fmt.Println("Validation passed")
	// Output: Validation passed
}

// Example_basicValidation demonstrates basic validation chain usage.
func Example_basicValidation() {
	// Create a header validator with common rules
	headerVal, err := validation.CommonValidator[*testv1.TestResource](
		validation.CommonValidatorConfig{
			ExpectedKind:    "test",
			AllowedVersions: []string{"v1"},
			NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create validator: %v", err))
	}

	// Create a spec validator
	specVal := validation.SpecValidator(
		func(r *testv1.TestResource) error {
			if r.GetSpec() == nil {
				return sentinel.BadParameter("spec is required")
			}
			return nil
		},
	)

	// Combine validators using Chain
	completeValidator := validation.Chain(
		validation.CheckAndSetDefaultsValidator[*testv1.TestResource](),
		headerVal,
		specVal,
	)

	// Validate a resource
	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "my-test-resource"},
		Spec:     &testv1.TestResourceSpec{},
	}

	if err := completeValidator(resource); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		return
	}

	fmt.Println("Validation passed")
	// Output: Validation passed
}
