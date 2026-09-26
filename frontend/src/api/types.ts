import type { TaktischesZeichen } from '@taktische-zeichen/core';

export interface User {
  id: number;
  username: string;
  isAdmin: boolean;
  /** Linked to an SSO identity. */
  sso: boolean;
  /** Can sign in with a password (SSO-only accounts can't). */
  hasPassword: boolean;
  /** The administrator role is synced from the SSO provider's groups. */
  adminManaged: boolean;
  /** The SSO provider's groups as of the last sign-in, by name; empty for local accounts. */
  groups: GroupRef[];
  createdAt: string;
  updatedAt: string;
}

/** Result of POST /api/auth/login. */
export interface LoginResponse {
  /** Access token, sent as `Authorization: Bearer <token>`. */
  token: string;
  tokenType: 'Bearer';
  expiresAt: string;
  user: User;
}

export interface AuthMethods {
  oidc: boolean;
  /** Label for the SSO button, e.g. the provider's name. */
  oidcName?: string;
}

export interface CreateUserInput {
  username: string;
  password: string;
  isAdmin: boolean;
}

export interface UpdateUserInput {
  isAdmin: boolean;
  /** Empty keeps the current password. */
  password: string;
}

export interface CreateTokenInput {
  /** Identifies the token, e.g. the device using it; 1 to 64 characters. */
  name: string;
  /** How long the token is valid, 60 seconds to 10 years. */
  ttlSeconds: number;
}

/** An API token issued by an administrator; its value is not kept. */
export interface ApiToken {
  id: number;
  name: string;
  createdAt: string;
  expiresAt: string;
}

/** Result of POST /api/users/{id}/tokens: the only time the value is returned. */
export interface CreatedApiToken extends ApiToken {
  /** Access token, sent as `Authorization: Bearer <token>`. */
  token: string;
  tokenType: 'Bearer';
}

/** A user referenced by a record; the record keeps null once they're deleted. */
export interface InstanceInfo {
  /** Name of this deployment; absent when not configured. */
  name?: string;
}

export interface VersionInfo {
  /** The release tag; absent for untagged builds. */
  version?: string;
  commit: string;
}

export interface GroupRef {
  id: number;
  name: string;
}

/** A group at the SSO provider, created when a member first signs in. */
export interface Group {
  id: number;
  name: string;
  /** By username; empty once the last member has left. */
  members: UserRef[];
  createdAt: string;
  updatedAt: string;
}

export interface UserRef {
  id: number;
  username: string;
}

export interface Position {
  /** WGS 84 degrees. */
  lat: number;
  lon: number;
  /** Meters; null when unknown. */
  height: number | null;
  /** Horizontal accuracy radius in meters; null when unknown. */
  accuracy: number | null;
  /** Speed over ground in meters per second; null when unknown. */
  speed: number | null;
  /** Course over ground in degrees clockwise from true north; null when unknown. */
  course: number | null;
  /** When the position was measured. */
  timestamp: string;
}

/** One entry of a unit's position history (GET /api/units/{id}/positions). */
export interface PositionHistoryEntry extends Position {
  /** When the entry was recorded. */
  recordedAt: string;
  /** Who set the position; null once that user is deleted. */
  recordedBy: UserRef | null;
}

/** A tactical symbol (DV 102), as component IDs of @taktische-zeichen/core. */
export type UnitSymbol = Pick<
  TaktischesZeichen,
  | 'grundzeichen'
  | 'organisation'
  | 'fachaufgabe'
  | 'einheit'
  | 'verwaltungsstufe'
  | 'funktion'
  | 'symbol'
>;

/** A radio call sign split into its parts, e.g. "Rotkreuz Musterstadt 12/83-1". */
export interface TacticalName {
  organisation?: string;
  regionalAssociation?: string;
  localAssociation?: string;
  function?: string;
  number?: string;
}

export interface Unit {
  id: string;
  name: string;
  position: Position | null;
  symbol: UnitSymbol | null;
  tacticalName: TacticalName | null;
  createdAt: string;
  updatedAt: string;
  createdBy: UserRef | null;
  updatedBy: UserRef | null;
}

export interface UnitInput {
  name: string;
  /** Null clears the position. Omitting `timestamp` means "now". */
  position: (Omit<Position, 'timestamp'> & { timestamp?: string }) | null;
  /** Null clears the symbol. */
  symbol: UnitSymbol | null;
  /** Null clears the tactical name. */
  tacticalName: TacticalName | null;
}

/** A message on the unit event stream (GET /api/units/events). */
export type UnitEvent =
  { type: 'created' | 'updated'; id: string; unit: Unit } | { type: 'deleted'; id: string };

export const AUDIT_ACTIONS = [
  'auth.login',
  'auth.login_failed',
  'auth.logout',
  'user.create',
  'user.update',
  'user.delete',
  'token.create',
  'token.revoke',
  'unit.create',
  'unit.update',
  'unit.delete',
] as const;

export type AuditAction = (typeof AUDIT_ACTIONS)[number];

/** One entry of the audit log (GET /api/audit-log). */
export interface AuditLogEntry {
  id: number;
  createdAt: string;
  action: AuditAction;
  /** Null for anonymous requests and once the user is deleted. */
  actor: UserRef | null;
  /** The actor's username at the time; for a failed sign-in, the one tried. */
  actorName: string;
  /** Empty for sign-ins and sign-outs. */
  targetType: '' | 'user' | 'token' | 'unit';
  targetId: string;
  targetName: string;
  /** Action-specific values, e.g. `changed` for unit updates. */
  details: Record<string, unknown>;
  remoteAddr: string;
}

export interface AuditLogQuery {
  action?: AuditAction;
  /** Only entries older than the one with this ID, to load the next page. */
  before?: number;
  limit?: number;
}
