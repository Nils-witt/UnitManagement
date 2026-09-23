import type { TaktischesZeichen } from '@taktische-zeichen/core';

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
