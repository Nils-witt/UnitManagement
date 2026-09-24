import { useMemo, useState } from 'react';
import { Box, TextField, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import ErrorBanner from '../components/ErrorBanner';
import { useGroups } from '../hooks/useGroups';
import GroupsListCard from './groups-page/GroupsListCard';
import { filterGroups } from './groups-page/groupFilter';
import './users-page/UsersToolbar.scss';

/** Groups come from the SSO provider, so this page only lists them. */
export default function GroupsPage() {
  const { t } = useTranslation();
  const { groups, loading, error } = useGroups();
  const [search, setSearch] = useState('');

  const visibleGroups = useMemo(() => filterGroups(groups, search), [groups, search]);

  return (
    <Box>
      <div className="users-toolbar">
        <TextField
          label={t('groups.searchLabel')}
          size="small"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Typography variant="body2" color="text.secondary">
          {t('groups.syncedNotice')}
        </Typography>
      </div>
      <ErrorBanner message={error} />
      <GroupsListCard groups={visibleGroups} loading={loading} />
    </Box>
  );
}
