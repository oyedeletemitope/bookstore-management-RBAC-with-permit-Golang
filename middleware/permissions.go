// permission.go
package middleware

import (
	"database/sql"
	"fmt"

	"github.com/permitio/permit-golang/pkg/enforcement"
	"github.com/permitio/permit-golang/pkg/permit"
)

type PermissionChecker struct {
	permitClient *permit.Client
	db           *sql.DB
}

func NewPermissionChecker(permitClient *permit.Client, db *sql.DB) *PermissionChecker {
	return &PermissionChecker{
		permitClient: permitClient,
		db:           db,
	}
}

func (pc *PermissionChecker) CheckPermission(username, action string) (enforcement.User, error) {
	role, err := GetUserRole(pc.db, username)
	if err != nil {
		return enforcement.User{}, fmt.Errorf("role lookup error: %w", err)
	}

	user := enforcement.UserBuilder(username).
		WithAttributes(map[string]interface{}{"role": role}).
		Build()
	resource := enforcement.ResourceBuilder("books").WithTenant("default").Build()

	permitted, err := pc.permitClient.Check(user, enforcement.Action(action), resource)
	if err != nil || !permitted {
		return enforcement.User{}, fmt.Errorf("access denied for user %s with role %s", username, role)
	}

	return user, nil
}
