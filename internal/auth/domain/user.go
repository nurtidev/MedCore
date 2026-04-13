package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleDoctor      Role = "doctor"
	RoleCoordinator Role = "coordinator"
	RoleAdmin       Role = "admin"
	RoleSuperAdmin  Role = "super_admin"
)

type Permission string

const (
	PermViewPatients  Permission = "patients:read"
	PermEditPatients  Permission = "patients:write"
	PermViewSchedule  Permission = "schedule:read"
	PermEditSchedule  Permission = "schedule:write"
	PermViewBilling   Permission = "billing:read"
	PermManageBilling Permission = "billing:manage"
	PermViewAnalytics Permission = "analytics:read"
	PermManageUsers   Permission = "users:manage"
	PermManageClinics Permission = "clinics:manage"
)

var allPermissions = []Permission{
	PermViewPatients, PermEditPatients,
	PermViewSchedule, PermEditSchedule,
	PermViewBilling, PermManageBilling,
	PermViewAnalytics, PermManageUsers, PermManageClinics,
}

// DefaultRolePermissions — матрица прав из ТЗ DCH.
var DefaultRolePermissions = map[Role][]Permission{
	RoleDoctor:      {PermViewPatients, PermViewSchedule},
	RoleCoordinator: {PermViewPatients, PermEditPatients, PermViewSchedule, PermEditSchedule, PermViewBilling},
	RoleAdmin:       {PermViewPatients, PermEditPatients, PermViewSchedule, PermEditSchedule, PermViewBilling, PermManageBilling, PermViewAnalytics, PermManageUsers},
	RoleSuperAdmin:  allPermissions,
}

type User struct {
	ID           uuid.UUID
	ClinicID     uuid.UUID // B2B — привязка к клинике
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	IIN          string // зашифрован AES-256-GCM
	Phone        string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RegisterRequest struct {
	ClinicID  uuid.UUID
	Email     string
	Password  string
	FirstName string
	LastName  string
	IIN       string
	Phone     string
	Role      Role
}

type UpdateUserRequest struct {
	FirstName *string
	LastName  *string
	Phone     *string
}
