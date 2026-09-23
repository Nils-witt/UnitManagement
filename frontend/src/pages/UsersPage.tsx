import { useMemo, useState } from 'react';
import { Box, Button, TextField } from '@mui/material';
import PersonAddIcon from '@mui/icons-material/PersonAdd';
import { useTranslation } from 'react-i18next';
import type { User } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useApi } from '../hooks/useApi';
import { useAuth } from '../hooks/useAuth';
import { useConfirm } from '../hooks/useConfirm';
import { useUsers } from '../hooks/useUsers';
import { errorMessage } from '../lib/errors';
import type { SortDirection } from '../lib/sort';
import UserDialog, { type UserFormValues } from './users-page/UserDialog';
import UsersListCard from './users-page/UsersListCard';
import { filterUsers, sortUsers, type UserSortKey } from './users-page/userSort';
import './users-page/UsersToolbar.scss';

export default function UsersPage() {
  const { t } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const { users, loading, error: loadError, reloadUsers } = useUsers();
  const { user: currentUser } = useAuth();

  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState<UserSortKey>('username');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  // The edited user stays set while the dialog closes, so its form doesn't
  // flip to "new user" during the exit animation.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);

  const visibleUsers = useMemo(
    () => sortUsers(filterUsers(users, search), sortKey, sortDirection),
    [users, search, sortKey, sortDirection],
  );

  const onSort = (key: UserSortKey) => {
    if (key === sortKey) {
      setSortDirection((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      // Newest first is the useful default for dates.
      setSortDirection(key === 'createdAt' ? 'desc' : 'asc');
    }
  };

  const openDialog = (user: User | null) => {
    setEditingUser(user);
    setDialogOpen(true);
  };

  const onSubmit = async ({ username, password, isAdmin }: UserFormValues) => {
    if (editingUser) {
      await api.updateUser(editingUser.id, { isAdmin, password });
    } else {
      await api.createUser({ username, password, isAdmin });
    }
    await reloadUsers();
  };

  const onDelete = async (u: User) => {
    const ok = await confirm({
      title: t('users.deleteUserTitle'),
      message: t('users.deleteUserMessage', { username: u.username }),
      confirmLabel: t('common.delete'),
      destructive: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await api.deleteUser(u.id);
      await reloadUsers();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Box>
      <div className="users-toolbar">
        <TextField
          label={t('users.searchLabel')}
          size="small"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button variant="contained" startIcon={<PersonAddIcon />} onClick={() => openDialog(null)}>
          {t('users.newUser')}
        </Button>
      </div>
      <ErrorBanner message={error ?? loadError} />
      <UsersListCard
        users={visibleUsers}
        loading={loading}
        currentUserId={currentUser?.id}
        sortKey={sortKey}
        sortDirection={sortDirection}
        onSort={onSort}
        onEdit={openDialog}
        onDelete={(u) => void onDelete(u)}
      />
      <UserDialog
        open={dialogOpen}
        user={editingUser}
        self={editingUser != null && editingUser.id === currentUser?.id}
        onClose={() => setDialogOpen(false)}
        onSubmit={onSubmit}
      />
    </Box>
  );
}
