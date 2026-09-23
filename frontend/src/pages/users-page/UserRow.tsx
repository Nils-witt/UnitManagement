import { Button, Chip, Stack, TableCell, TableRow } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { User } from '../../api/types';
import { fmtDate } from '../../lib/format';
import './UserRow.scss';

export default function UserRow({
  u,
  self,
  onEdit,
  onDelete,
}: {
  u: User;
  self: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <TableRow hover>
      <TableCell>
        {u.username}
        {self ? ` (${t('userRow.you')})` : ''}
      </TableCell>
      <TableCell>
        <Stack direction="row" spacing={0.5} useFlexGap className="user-row__chips">
          {u.sso && <Chip size="small" color="info" variant="outlined" label={t('userRow.sso')} />}
          {u.hasPassword && <Chip size="small" variant="outlined" label={t('userRow.password')} />}
        </Stack>
      </TableCell>
      <TableCell>
        {u.isAdmin ? (
          <Chip size="small" color="primary" label={t('userRow.admin')} />
        ) : (
          t('userRow.user')
        )}
      </TableCell>
      <TableCell>{fmtDate(u.createdAt, i18n.language)}</TableCell>
      <TableCell className="user-row__actions">
        <Stack direction="row" spacing={1} useFlexGap className="user-row__action-buttons">
          <Button
            size="small"
            onClick={onEdit}
            aria-label={t('userRow.editAria', { name: u.username })}
          >
            {t('common.edit')}
          </Button>
          <Button
            size="small"
            color="error"
            disabled={self}
            title={self ? t('userRow.cantDeleteSelf') : undefined}
            aria-label={t('userRow.deleteAria', { name: u.username })}
            onClick={onDelete}
          >
            {t('common.delete')}
          </Button>
        </Stack>
      </TableCell>
    </TableRow>
  );
}
