import {
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import type { SortDirection } from '../../lib/sort';
import UnitRow from './UnitRow';
import type { UnitSortKey } from './unitSort';

export default function UnitsListCard({
  units,
  loading,
  sortKey,
  sortDirection,
  onSort,
  onEdit,
  onDelete,
}: {
  units: Unit[];
  loading: boolean;
  sortKey: UnitSortKey;
  sortDirection: SortDirection;
  onSort: (key: UnitSortKey) => void;
  onEdit: (u: Unit) => void;
  onDelete: (u: Unit) => void;
}) {
  const { t } = useTranslation();

  const sortCell = (key: UnitSortKey, label: string) => (
    <TableCell sortDirection={sortKey === key ? sortDirection : false}>
      <TableSortLabel
        active={sortKey === key}
        direction={sortKey === key ? sortDirection : 'asc'}
        onClick={() => onSort(key)}
      >
        {label}
      </TableSortLabel>
    </TableCell>
  );

  return (
    <TableContainer component={Paper}>
      <Table size="small">
        <TableHead>
          <TableRow>
            {sortCell('name', t('unitsListCard.name'))}
            <TableCell>{t('unitsListCard.position')}</TableCell>
            {sortCell('updatedAt', t('unitsListCard.updated'))}
            <TableCell>{t('unitsListCard.created')}</TableCell>
            <TableCell>{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {units.map((u) => (
            <UnitRow key={u.id} u={u} onEdit={() => onEdit(u)} onDelete={() => onDelete(u)} />
          ))}
          {units.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} align="center">
                {loading ? t('common.loading') : t('unitsListCard.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
