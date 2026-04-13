import type { Role } from '@/types/auth'

const roleHierarchy: Record<Role, number> = {
  doctor: 1,
  coordinator: 2,
  admin: 3,
  super_admin: 4,
}

export function hasRole(userRole: Role | undefined, required: Role[]): boolean {
  if (!userRole) return false
  return required.includes(userRole)
}

export function isAtLeast(userRole: Role | undefined, minRole: Role): boolean {
  if (!userRole) return false
  return roleHierarchy[userRole] >= roleHierarchy[minRole]
}

export const ADMIN_ROLES: Role[] = ['admin', 'super_admin']
export const ALL_ROLES: Role[] = ['doctor', 'coordinator', 'admin', 'super_admin']
