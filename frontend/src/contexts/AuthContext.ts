import { createContext } from 'react';
import type { User } from '../api/types';

export interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  /** Set when the session ended on its own (expired or revoked), so the login
   * page can say why. Cleared by `logout` and `clearSessionMessage`. */
  sessionMessage: string | null;
  /** Performs POST /api/auth/login and stores the access token. */
  login: (username: string, password: string) => Promise<User>;
  /** Ends the session on the user's request. */
  logout: () => Promise<void>;
  clearSessionMessage: () => void;
  /** Ends a session the server no longer accepts and remembers why in
   * `sessionMessage`. Does nothing if there is no session. Stable across
   * renders. */
  expireSession: () => void;
}

export const AuthContext = createContext<AuthState | null>(null);
