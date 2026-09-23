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
