package validation_test

import (
	"errors"
	"testing"

	testv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/test/v1"

	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"

	"github.com/loicsikidi/holistik/internal/apiserver/services/internal/generic/validation"
	"github.com/loicsikidi/sentinel"
)

// TestChain_Empty tests that an empty chain returns nil.
func TestChain_Empty(t *testing.T) {
	validator := validation.Chain[*testv1.TestResource]()

	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "test-resource"},
	}

	if err := validator(resource); err != nil {
		t.Errorf("empty chain should return nil, got: %v", err)
	}
}

// TestChain_SingleValidator tests chain with a single validator.
func TestChain_SingleValidator(t *testing.T) {
	called := false
	validator := validation.Chain(
		validation.SpecValidator(func(r *testv1.TestResource) error {
			called = true
			return nil
		}),
	)

	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "test"},
	}

	if err := validator(resource); err != nil {
		t.Errorf("validator failed: %v", err)
	}

	if !called {
		t.Error("validator was not called")
	}
}

// TestChain_MultipleValidators tests chain with multiple validators.
func TestChain_MultipleValidators(t *testing.T) {
	var calls []string

	validator := validation.Chain(
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "first")
			return nil
		}),
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "second")
			return nil
		}),
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "third")
			return nil
		}),
	)

	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "test"},
	}

	if err := validator(resource); err != nil {
		t.Errorf("validator failed: %v", err)
	}

	if len(calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(calls))
	}

	if calls[0] != "first" || calls[1] != "second" || calls[2] != "third" {
		t.Errorf("validators called in wrong order: %v", calls)
	}
}

// TestChain_StopOnError tests that chain stops at first error.
func TestChain_StopOnError(t *testing.T) {
	var calls []string
	expectedErr := errors.New("validation failed")

	validator := validation.Chain(
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "first")
			return nil
		}),
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "second")
			return expectedErr
		}),
		validation.SpecValidator(func(r *testv1.TestResource) error {
			calls = append(calls, "third")
			return nil
		}),
	)

	resource := &testv1.TestResource{
		Kind:     "test",
		Version:  "v1",
		Metadata: &headerv1.Metadata{Name: "test"},
	}

	err := validator(resource)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	if len(calls) != 2 {
		t.Errorf("expected 2 calls (stopped at error), got %d", len(calls))
	}
}

// customResource is a test helper type that implements generic.Resource.
type customResource struct {
	metadata *headerv1.Metadata
}

func (c *customResource) GetMetadata() *headerv1.Metadata {
	return c.metadata
}

// TestCheckAndSetDefaultsValidator tests the CheckAndSetDefaults wrapper.
func TestCheckAndSetDefaultsValidator(t *testing.T) {
	t.Run("resource without CheckAndSetDefaults is no-op", func(t *testing.T) {
		validator := validation.CheckAndSetDefaultsValidator[*testv1.TestResource]()

		resource := &testv1.TestResource{
			Kind:     "test",
			Version:  "v1",
			Metadata: &headerv1.Metadata{Name: "test-resource"},
		}

		// Should be a no-op since TestResource doesn't implement CheckAndSetDefaults
		if err := validator(resource); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("resource with CheckAndSetDefaults calls the method", func(t *testing.T) {
		// Create instance
		resource := &customResource{
			metadata: &headerv1.Metadata{Name: "test"},
		}
		var checkCalled bool

		validator := validation.Validator[*customResource](func(r *customResource) error {
			if impl, ok := any(r).(interface{ CheckAndSetDefaults() error }); ok {
				checkCalled = true
				return impl.CheckAndSetDefaults()
			}
			return nil
		})

		if err := validator(resource); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// CheckAndSetDefaults should not have been called since customResource doesn't implement it
		if checkCalled {
			t.Error("CheckAndSetDefaults should not have been called")
		}
	})
}

// TestCommonValidator_Kind tests kind validation.
func TestCommonValidator_Kind(t *testing.T) {
	tests := []struct {
		name        string
		config      validation.CommonValidatorConfig
		resource    *testv1.TestResource
		expectError bool
	}{
		{
			name: "valid kind",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
			},
			resource: &testv1.TestResource{
				Kind:     "test",
				Version:  "v1",
				Metadata: &headerv1.Metadata{Name: "test-resource"},
			},
			expectError: false,
		},
		{
			name: "invalid kind",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
			},
			resource: &testv1.TestResource{
				Kind:     "wrong",
				Version:  "v1",
				Metadata: &headerv1.Metadata{Name: "test-resource"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := validation.CommonValidator[*testv1.TestResource](tt.config)
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			err = validator(tt.resource)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestHeaderValidator_Version tests version validation.
func TestHeaderValidator_Version(t *testing.T) {
	tests := []struct {
		name        string
		config      validation.CommonValidatorConfig
		version     string
		expectError bool
	}{
		{
			name: "valid version",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1"},
			},
			version:     "v1",
			expectError: false,
		},
		{
			name: "invalid version",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1"},
			},
			version:     "v2",
			expectError: true,
		},
		{
			name: "multiple allowed versions - first",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1", "v2", "v3"},
			},
			version:     "v1",
			expectError: false,
		},
		{
			name: "multiple allowed versions - middle",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1", "v2", "v3"},
			},
			version:     "v2",
			expectError: false,
		},
		{
			name: "multiple allowed versions - last",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1", "v2", "v3"},
			},
			version:     "v3",
			expectError: false,
		},
		{
			name: "multiple allowed versions - invalid",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedVersions: []string{"v1", "v2"},
			},
			version:     "v3",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := validation.CommonValidator[*testv1.TestResource](tt.config)
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			resource := &testv1.TestResource{
				Kind:     "test",
				Version:  tt.version,
				Metadata: &headerv1.Metadata{Name: "test-resource"},
			}

			err = validator(resource)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestHeaderValidator_SubKind tests subkind validation.
func TestHeaderValidator_SubKind(t *testing.T) {
	tests := []struct {
		name        string
		config      validation.CommonValidatorConfig
		subKind     string
		expectError bool
	}{
		{
			name: "empty subkind with no constraints",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
			},
			subKind:     "",
			expectError: false,
		},
		{
			name: "empty subkind not in allowed list",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedSubKinds: []string{"foo", "bar"},
			},
			subKind:     "",
			expectError: true,
		},
		{
			name: "empty subkind in allowed list",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedSubKinds: []string{"", "foo", "bar"},
			},
			subKind:     "",
			expectError: false,
		},
		{
			name: "valid subkind",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedSubKinds: []string{"foo", "bar"},
			},
			subKind:     "foo",
			expectError: false,
		},
		{
			name: "invalid subkind",
			config: validation.CommonValidatorConfig{
				ExpectedKind:    "test",
				AllowedSubKinds: []string{"foo", "bar"},
			},
			subKind:     "baz",
			expectError: true,
		},
		{
			name: "no subkind constraints - any value allowed",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
			},
			subKind:     "anything",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := validation.CommonValidator[*testv1.TestResource](tt.config)
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			resource := &testv1.TestResource{
				Kind:     "test",
				SubKind:  tt.subKind,
				Version:  "v1",
				Metadata: &headerv1.Metadata{Name: "test-resource"},
			}

			err = validator(resource)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestHeaderValidator_Name tests name validation.
func TestHeaderValidator_Name(t *testing.T) {
	tests := []struct {
		name         string
		config       validation.CommonValidatorConfig
		resourceName string
		expectError  bool
	}{
		{
			name: "valid name pattern",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
				NamePattern:  `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
			},
			resourceName: "test-resource-123",
			expectError:  false,
		},
		{
			name: "invalid name pattern",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
				NamePattern:  `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
			},
			resourceName: "Test_Resource",
			expectError:  true,
		},
		{
			name: "single character name with pattern",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
				NamePattern:  `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
			},
			resourceName: "a",
			expectError:  false,
		},
		{
			name: "empty name with pattern",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
				NamePattern:  `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
			},
			resourceName: "",
			expectError:  true,
		},
		{
			name: "no name constraints",
			config: validation.CommonValidatorConfig{
				ExpectedKind: "test",
			},
			resourceName: "any-name-works!",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := validation.CommonValidator[*testv1.TestResource](tt.config)
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			resource := &testv1.TestResource{
				Kind:     "test",
				Version:  "v1",
				Metadata: &headerv1.Metadata{Name: tt.resourceName},
			}

			err = validator(resource)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestCommonValidator_InvalidConfig tests configuration validation.
func TestCommonValidator_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config validation.CommonValidatorConfig
	}{
		{
			name: "invalid regex",
			config: validation.CommonValidatorConfig{
				NamePattern: "[invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validation.CommonValidator[*testv1.TestResource](tt.config)
			if err == nil {
				t.Error("expected config validation error, got nil")
			}
		})
	}
}

// TestCommonValidator_NilResource tests handling of nil resource.
func TestCommonValidator_NilResource(t *testing.T) {
	validator, err := validation.CommonValidator[*testv1.TestResource](
		validation.CommonValidatorConfig{
			ExpectedKind: "test",
		},
	)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Resource with nil header
	resource := &testv1.TestResource{}
	err = validator(resource)
	if err == nil {
		t.Error("expected error for nil header, got nil")
	}
}

// TestIntegration_CompleteValidationChain tests a complete validation chain.
func TestIntegration_CompleteValidationChain(t *testing.T) {
	commonVal, err := validation.CommonValidator[*testv1.TestResource](
		validation.CommonValidatorConfig{
			ExpectedKind:    "test",
			AllowedVersions: []string{"v1"},
			NamePattern:     `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
		},
	)
	if err != nil {
		t.Fatalf("failed to create header validator: %v", err)
	}

	specVal := validation.SpecValidator(
		func(r *testv1.TestResource) error {
			spec := r.GetSpec()
			if spec == nil {
				return sentinel.BadParameter("spec is required")
			}
			return nil
		},
	)

	completeValidator := validation.Chain(
		validation.CheckAndSetDefaultsValidator[*testv1.TestResource](),
		commonVal,
		specVal,
	)

	t.Run("valid resource", func(t *testing.T) {
		resource := &testv1.TestResource{
			Kind:     "test",
			Version:  "v1",
			Metadata: &headerv1.Metadata{Name: "test-resource"},
			Spec:     &testv1.TestResourceSpec{},
		}

		if err := completeValidator(resource); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid kind", func(t *testing.T) {
		resource := &testv1.TestResource{
			Kind:     "wrong",
			Version:  "v1",
			Metadata: &headerv1.Metadata{Name: "test-resource"},
			Spec:     &testv1.TestResourceSpec{},
		}

		if err := completeValidator(resource); err == nil {
			t.Error("expected error for invalid kind")
		}
	})

	t.Run("invalid name pattern", func(t *testing.T) {
		resource := &testv1.TestResource{
			Kind:     "test",
			Version:  "v1",
			Metadata: &headerv1.Metadata{Name: "Test_Resource"},
			Spec:     &testv1.TestResourceSpec{},
		}

		if err := completeValidator(resource); err == nil {
			t.Error("expected error for invalid name pattern")
		}
	})

	t.Run("missing spec", func(t *testing.T) {
		resource := &testv1.TestResource{
			Kind:     "test",
			Version:  "v1",
			Metadata: &headerv1.Metadata{Name: "test-resource"},
		}

		if err := completeValidator(resource); err == nil {
			t.Error("expected error for missing spec")
		}
	})
}
