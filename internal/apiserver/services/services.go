package services

import "github.com/loicsikidi/sentinel"

// CheckAndSetDefaults calls [r.CheckAndSetDefaults] if r implements the method.
// If r does not implement, then this is a nop.
func CheckAndSetDefaults(r any) error {
	if r, ok := r.(interface{ CheckAndSetDefaults() error }); ok {
		return sentinel.Wrap(r.CheckAndSetDefaults())
	}

	return nil
}
