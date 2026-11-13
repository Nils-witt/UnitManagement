import {
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Group } from '../../api/types';
import { fmtDate } from '../../lib/format';
import '../users-page/UserRow.scss';

export default function GroupsListCard({ groups, loading }: { groups: Group[]; loading: boolean }) {
  const { t, i18n } = useTranslation();
  return (
    <TableContainer component={Paper}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('groupsListCard.name')}</TableCell>
            <TableCell>{t('groupsListCard.members')}</TableCell>
            <TableCell>{t('groupsListCard.units')}</TableCell>
            <TableCell>{t('groupsListCard.firstSeen')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {groups.map((g) => (
            <TableRow key={g.id} hover>
              <TableCell>{g.name}</TableCell>
              <TableCell>
                {g.members.length === 0 ? (
                  t('groupsListCard.noMembers')
                ) : (
                  <Stack direction="row" spacing={0.5} useFlexGap className="user-row__chips">
                    {g.members.map((m) => (
                      <Chip key={m.id} size="small" variant="outlined" label={m.username} />
                    ))}
                  </Stack>
                )}
              </TableCell>
              <TableCell>
                {g.units.length === 0 ? (
                  t('groupsListCard.noUnits')
                ) : (
                  <Stack direction="row" spacing={0.5} useFlexGap className="user-row__chips">
                    {g.units.map((u) => (
                      <Chip key={u.id} size="small" variant="outlined" label={u.name} />
                    ))}
                  </Stack>
                )}
              </TableCell>
              <TableCell>{fmtDate(g.createdAt, i18n.language)}</TableCell>
            </TableRow>
          ))}
          {groups.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} align="center">
                {loading ? t('common.loading') : t('groupsListCard.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
