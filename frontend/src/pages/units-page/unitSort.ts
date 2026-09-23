import type { Unit } from '../../api/types';
import { sortBy, type SortDirection } from '../../lib/sort';
import { formatTacticalName } from '../../lib/tacticalName';

export type UnitSortKey = 'name' | 'updatedAt';

export function sortUnits(units: Unit[], key: UnitSortKey, direction: SortDirection): Unit[] {
  return sortBy(
    units,
    direction,
    (u) => (key === 'name' ? u.name : Date.parse(u.updatedAt)),
    (u) => u.name,
  );
}

export function filterUnits(units: Unit[], search: string): Unit[] {
  const needle = search.trim().toLowerCase();
  if (!needle) return units;
  return units.filter(
    (u) =>
      u.name.toLowerCase().includes(needle) ||
      formatTacticalName(u.tacticalName).toLowerCase().includes(needle),
  );
}
