package harness

import (
	"testing"
)

func assertPermission(t *testing.T, expected Permission, actual func() Permission) {
	t.Helper()

	got := actual()
	if expected != got {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestPermission(t *testing.T) {
	t.Run("multiple allow in one call", func(t *testing.T) {
		var expectedPermission Permission
		expectedPermission.Allow(PermissionEdit)
		expectedPermission.Allow(PermissionRead)

		assertPermission(t, expectedPermission, func() Permission {
			var p Permission
			p.Allow(PermissionEdit | PermissionRead)
			return p
		})
	})

	t.Run("deny multiple permissions", func(t *testing.T) {
		var expectedPermission Permission
		expectedPermission.Allow(PermissionAll)
		expectedPermission.Deny(PermissionBashCustom | PermissionBashUnsafe)

		assertPermission(t, expectedPermission, func() Permission {
			var p Permission
			p.Allow(PermissionAll)
			p.Deny(PermissionBashCustom | PermissionBashUnsafe)
			return p
		})
	})

	t.Run("empty permission denies all permissions", func(t *testing.T) {
		assertPermission(t, Permission(0), func() Permission {
			var p Permission
			return p
		})
	})

	t.Run("allow all permissions", func(t *testing.T) {
		assertPermission(t, PermissionAll, func() Permission {
			var p Permission
			p.Allow(PermissionAll)
			return p
		})
	})

	t.Run("deny all permissions", func(t *testing.T) {
		assertPermission(t, Permission(0), func() Permission {
			var p Permission
			p.Allow(PermissionEdit | PermissionRead)
			p.Deny(PermissionAll)
			return p
		})
	})
}
