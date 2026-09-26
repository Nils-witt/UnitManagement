// The one place that talks to the Go HTTP API. It owns the request plumbing
// (the access token, JSON bodies, error mapping) and one typed method per
// endpoint, so callers never build URLs or serialize bodies themselves.
//
// Every request carries the stored access token as a Bearer header; a 401 is
// reported through `onSessionExpired`.

import { getToken } from './tokenStore';
import type {
  ApiToken,
  AuditLogEntry,
  AuditLogQuery,
  AuthMethods,
  CreateTokenInput,
  CreateUserInput,
  CreatedApiToken,
  Group,
  InstanceInfo,
  LoginResponse,
  PositionHistoryEntry,
  Unit,
  UnitInput,
  UpdateUserInput,
  User,
  VersionInfo,
} from './types';

export class ApiError extends Error {
  /** HTTP status of the failed response; 0 if there was none. */
  readonly status: number;

  constructor(message: string, status = 0) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export interface ApiClientOptions {
  /** Called when the server rejects the access token. */
  onSessionExpired?: () => void;
}

export class ApiClient {
  private options: ApiClientOptions;

  constructor(options: ApiClientOptions = {}) {
    this.options = options;
  }

  // ---- request plumbing --------------------------------------------------

  /** Throws an ApiError carrying the server's `{"error": "..."}` message and
   * the status on any non-OK response. */
  private async request(path: string, init: RequestInit = {}): Promise<Response> {
    const headers = new Headers(init.headers);
    const token = getToken();
    if (token) headers.set('Authorization', `Bearer ${token}`);

    let res: Response;
    try {
      res = await fetch(path, { ...init, headers });
    } catch {
      throw new ApiError('network error');
    }

    if (!res.ok) {
      if (res.status === 401) this.options.onSessionExpired?.();
      const body = (await res.json().catch(() => null)) as { error?: string } | null;
      throw new ApiError(body?.error || `request failed with status ${res.status}`, res.status);
    }

    return res;
  }

  private async getJson<T>(path: string): Promise<T> {
    const res = await this.request(path);
    return (await res.json()) as T;
  }

  private sendJson(path: string, method: string, body?: unknown): Promise<Response> {
    return this.request(path, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  }

  private async del(path: string): Promise<void> {
    await this.request(path, { method: 'DELETE' });
  }

  private async sendJsonForJson<T>(path: string, method: string, body: unknown): Promise<T> {
    const res = await this.sendJson(path, method, body);
    return (await res.json()) as T;
  }

  // ---- auth --------------------------------------------------------------

  /** POST /api/auth/login. Resolves to the access token and the user; the
   * caller stores the token. Rejects with status 401 for wrong credentials. */
  login(username: string, password: string): Promise<LoginResponse> {
    return this.sendJsonForJson('/api/auth/login', 'POST', { username, password });
  }

  /** Ends the session of the stored token on the server. */
  async logout(): Promise<void> {
    await this.sendJson('/api/auth/logout', 'POST');
  }

  /** The signed-in user, or null without a valid session. */
  async me(): Promise<User | null> {
    try {
      return await this.getJson<User>('/api/auth/me');
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return null;
      throw err;
    }
  }

  /** Which sign-in options the login page offers. */
  getAuthMethods(): Promise<AuthMethods> {
    return this.getJson('/api/auth/methods');
  }

  /** Full-page URL that starts SSO and comes back to `redirect` afterwards. */
  oidcLoginUrl(redirect: string): string {
    return '/api/auth/oidc/login?redirect=' + encodeURIComponent(redirect);
  }

  // ---- build info ------------------------------------------------------------

  /** GET /api/version. Public, so the login page footer can show it too. */
  getVersion(): Promise<VersionInfo> {
    return this.getJson('/api/version');
  }

  /** GET /api/instance. Public, so the login page can show the name too. */
  getInstance(): Promise<InstanceInfo> {
    return this.getJson('/api/instance');
  }

  // ---- users (administrators only) ----------------------------------------

  listUsers(): Promise<User[]> {
    return this.getJson('/api/users');
  }

  createUser(input: CreateUserInput): Promise<User> {
    return this.sendJsonForJson('/api/users', 'POST', input);
  }

  updateUser(id: number, input: UpdateUserInput): Promise<User> {
    return this.sendJsonForJson(`/api/users/${id}`, 'PUT', input);
  }

  deleteUser(id: number): Promise<void> {
    return this.del(`/api/users/${id}`);
  }

  /** The user's unexpired API tokens, newest first. */
  listUserTokens(id: number): Promise<ApiToken[]> {
    return this.getJson(`/api/users/${id}/tokens`);
  }

  /** Issues an access token in the user's name; the token is only returned here. */
  createUserToken(id: number, input: CreateTokenInput): Promise<CreatedApiToken> {
    return this.sendJsonForJson(`/api/users/${id}/tokens`, 'POST', input);
  }

  revokeUserToken(id: number, tokenId: number): Promise<void> {
    return this.del(`/api/users/${id}/tokens/${tokenId}`);
  }

  // ---- groups (administrators only) ---------------------------------------

  listGroups(): Promise<Group[]> {
    return this.getJson('/api/groups');
  }

  // ---- audit log (administrators only) -------------------------------------

  /** Audit log entries, newest first. */
  listAuditLog(query: AuditLogQuery = {}): Promise<AuditLogEntry[]> {
    const params = new URLSearchParams();
    if (query.action) params.set('action', query.action);
    if (query.before) params.set('before', String(query.before));
    if (query.limit) params.set('limit', String(query.limit));
    const qs = params.size > 0 ? `?${params}` : '';
    return this.getJson(`/api/audit-log${qs}`);
  }

  // ---- units ---------------------------------------------------------------

  listUnits(): Promise<Unit[]> {
    return this.getJson('/api/units');
  }

  createUnit(input: UnitInput): Promise<Unit> {
    return this.sendJsonForJson('/api/units', 'POST', input);
  }

  updateUnit(id: string, input: UnitInput): Promise<Unit> {
    return this.sendJsonForJson(`/api/units/${id}`, 'PUT', input);
  }

  /** The unit's position history, newest measurement first; `since` and `to`
   * (RFC 3339) limit it to positions measured in that timeframe. */
  listUnitPositions(
    id: string,
    range: { since?: string; to?: string } = {},
  ): Promise<PositionHistoryEntry[]> {
    const params = new URLSearchParams();
    if (range.since) params.set('since', range.since);
    if (range.to) params.set('to', range.to);
    const query = params.size > 0 ? `?${params}` : '';
    return this.getJson(`/api/units/${id}/positions${query}`);
  }

  deleteUnit(id: string): Promise<void> {
    return this.del(`/api/units/${id}`);
  }

  /** Opens the WebSocket that pushes every unit change (UnitEvent JSON
   * messages). Browsers can't set headers on a WebSocket, so the token goes
   * in the subprotocols "bearer, <token>" instead. */
  openUnitEvents(): WebSocket {
    const url = new URL('/api/units/events', window.location.href);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = getToken();
    return new WebSocket(url, token ? ['bearer', token] : undefined);
  }
}
