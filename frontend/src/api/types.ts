export interface User {
  id: number;
  username: string;
  isAdmin: boolean;
  /** Linked to an SSO identity. */
  sso: boolean;
  /** Can sign in with a password (SSO-only accounts can't). */
  hasPassword: boolean;
  createdAt: string;
  updatedAt: string;
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

/** A user referenced by a record; the record keeps null once they're deleted. */
export interface VersionInfo {
  /** The release tag; absent for untagged builds. */
  version?: string;
  commit: string;
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
  /** When the position was measured. */
  timestamp: string;
}

export interface Unit {
  id: string;
  name: string;
  position: Position | null;
  createdAt: string;
  updatedAt: string;
  createdBy: UserRef | null;
  updatedBy: UserRef | null;
}

export interface UnitInput {
  name: string;
  /** Null clears the position. Omitting `timestamp` means "now". */
  position: (Omit<Position, 'timestamp'> & { timestamp?: string }) | null;
}
