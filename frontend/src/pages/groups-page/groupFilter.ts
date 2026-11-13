import type { Group } from '../../api/types';

/** Matches a group by its name, a member's username or a unit's name. */
export function filterGroups(groups: Group[], search: string): Group[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return groups;
  return groups.filter(
    (g) =>
      g.name.toLowerCase().includes(needle) ||
      g.members.some((m) => m.username.toLowerCase().includes(needle)) ||
      g.units.some((u) => u.name.toLowerCase().includes(needle)),
  );
}
