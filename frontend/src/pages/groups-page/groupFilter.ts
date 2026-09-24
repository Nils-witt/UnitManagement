import type { Group } from '../../api/types';

/** Matches a group by its name or a member's username. */
export function filterGroups(groups: Group[], search: string): Group[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return groups;
  return groups.filter(
    (g) =>
      g.name.toLowerCase().includes(needle) ||
      g.members.some((m) => m.username.toLowerCase().includes(needle)),
  );
}
