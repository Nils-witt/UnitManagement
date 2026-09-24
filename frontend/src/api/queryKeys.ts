// Every query key in one place, so what is cached (and cleared on logout) is
// visible at a glance and two hooks can't collide on a key by accident.
export const queryKeys = {
  authMethods: ['auth', 'methods'] as const,
  instance: ['instance'] as const,
  users: ['users', 'list'] as const,
  units: ['units', 'list'] as const,
  unitPositions: (id: string) => ['units', 'positions', id] as const,
  version: ['version'] as const,
};
