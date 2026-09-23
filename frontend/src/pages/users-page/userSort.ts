import type { User } from '../../api/types';
import { sortBy, type SortDirection } from '../../lib/sort';

export type UserSortKey = 'username' | 'createdAt';

export function sortUsers(users: User[], key: UserSortKey, direction: SortDirection): User[] {
  return sortBy(
    users,
    direction,
    (u) => (key === 'username' ? u.username : Date.parse(u.createdAt)),
    (u) => u.username,
  );
}

export function filterUsers(users: User[], search: string): User[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return users;
  return users.filter((u) => u.username.toLowerCase().includes(needle));
}
