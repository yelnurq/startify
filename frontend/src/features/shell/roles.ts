import type { Role } from '@/features/auth/api';

/** Roles from the most to the least privileged, for showing one label per person. */
export const ROLE_ORDER: Role[] = [
  'tenant_admin',
  'innovation_manager',
  'leadership',
  'supervisor',
  'mentor',
  'jury',
  'partner',
  'student',
];

export function primaryRole(roles: readonly Role[]): Role | undefined {
  return ROLE_ORDER.find((role) => roles.includes(role));
}
