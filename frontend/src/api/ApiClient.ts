// The one place that talks to the Go HTTP API. It owns the request plumbing
// (cookies, JSON bodies, error mapping) and one typed method per endpoint, so
// callers never build URLs or serialize bodies themselves.
//
// The session lives in an HttpOnly cookie set by the server, so the client
// never sees a token: it only reports a 401 through `onSessionExpired`.

import type {
  AuthMethods,
  CreateUserInput,
  Group,
  InstanceInfo,
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
  /** Called when the server rejects the session cookie. */
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
    let res: Response;
    try {
      res = await fetch(path, { credentials: 'same-origin', ...init });
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

  /** POST /api/auth/login. The server sets the session cookie. Rejects with
   * status 401 for wrong credentials. */
  login(username: string, password: string): Promise<User> {
    return this.sendJsonForJson('/api/auth/login', 'POST', { username, password });
  }

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

  // ---- groups (administrators only) ---------------------------------------

  listGroups(): Promise<Group[]> {
    return this.getJson('/api/groups');
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

  /** The unit's position history, newest measurement first. */
  listUnitPositions(id: string): Promise<PositionHistoryEntry[]> {
    return this.getJson(`/api/units/${id}/positions`);
  }

  deleteUnit(id: string): Promise<void> {
    return this.del(`/api/units/${id}`);
  }

  /** Opens the WebSocket that pushes every unit change (UnitEvent JSON
   * messages). The session cookie authenticates it like any request. */
  openUnitEvents(): WebSocket {
    const url = new URL('/api/units/events', window.location.href);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return new WebSocket(url);
  }
}
