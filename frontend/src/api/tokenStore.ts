// Holds the access token (a JWT) the server issues at sign-in. It is kept in
// localStorage so a reload or a second tab stays signed in, like the session
// cookie did; if storage is unavailable (private mode, blocked site data) it
// lives in memory for this page only.

const STORAGE_KEY = 'accessToken';

let memoryToken: string | null = null;

export function getToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch {
    return memoryToken;
  }
}

export function setToken(token: string | null): void {
  memoryToken = token;
  try {
    if (token) localStorage.setItem(STORAGE_KEY, token);
    else localStorage.removeItem(STORAGE_KEY);
  } catch {
    // memoryToken already holds it.
  }
}

/** Takes the token that a finished SSO sign-in puts in the URL fragment
 * (#sso_token=...), removing it from the address bar and history entry. */
export function takeSsoToken(): string | null {
  const params = new URLSearchParams(window.location.hash.slice(1));
  const token = params.get('sso_token');
  if (!token) return null;
  const { pathname, search } = window.location;
  window.history.replaceState(window.history.state, '', pathname + search);
  return token;
}
