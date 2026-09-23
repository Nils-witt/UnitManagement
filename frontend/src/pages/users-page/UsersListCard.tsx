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
import type { User } from '../../api/types';
import type { SortDirection } from '../../lib/sort';
import UserRow from './UserRow';
import type { UserSortKey } from './userSort';

export default function UsersListCard({
  users,
  loading,
  currentUserId,
  sortKey,
  sortDirection,
  onSort,
  onEdit,
  onDelete,
}: {
  users: User[];
  loading: boolean;
  currentUserId: number | undefined;
  sortKey: UserSortKey;
  sortDirection: SortDirection;
  onSort: (key: UserSortKey) => void;
  onEdit: (u: User) => void;
  onDelete: (u: User) => void;
}) {
  const { t } = useTranslation();

  const sortCell = (key: UserSortKey, label: string) => (
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
            {sortCell('username', t('login.usernameLabel'))}
            <TableCell>{t('usersListCard.signIn')}</TableCell>
            <TableCell>{t('usersListCard.role')}</TableCell>
            {sortCell('createdAt', t('usersListCard.created'))}
            <TableCell>{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {users.map((u) => (
            <UserRow
              key={u.id}
              u={u}
              self={u.id === currentUserId}
              onEdit={() => onEdit(u)}
              onDelete={() => onDelete(u)}
            />
          ))}
          {users.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} align="center">
                {loading ? t('common.loading') : t('usersListCard.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
