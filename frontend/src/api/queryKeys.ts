// Every query key in one place, so what is cached (and cleared on logout) is
// visible at a glance and two hooks can't collide on a key by accident.
export const queryKeys = {
  authMethods: ['auth', 'methods'] as const,
  instance: ['instance'] as const,
  users: ['users', 'list'] as const,
  userTokens: (id: number) => ['users', 'tokens', id] as const,
  userApiKeys: (id: number) => ['users', 'apiKeys', id] as const,
  groups: ['groups', 'list'] as const,
  settings: ['settings'] as const,
  syncIdentity: ['sync', 'identity'] as const,
  syncRemotes: ['sync', 'remotes'] as const,
  syncRemoteLogs: (id: string) => ['sync', 'logs', id] as const,
  auditLog: (action: string | null) => ['auditLog', action] as const,
  units: ['units', 'list'] as const,
  // Prefix of every history of the unit, whatever its timeframe.
  unitPositions: (id: string) => ['units', 'positions', id] as const,
  unitPositionsRange: (id: string, since: string | null, to: string | null) =>
    ['units', 'positions', id, since, to] as const,
  version: ['version'] as const,
};
