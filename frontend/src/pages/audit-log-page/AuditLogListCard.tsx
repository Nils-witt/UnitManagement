import {
  Chip,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import type { AuditLogEntry } from '../../api/types';
import { fmtDate } from '../../lib/format';
import './AuditLogPage.scss';

type ChipColor = 'default' | 'success' | 'error' | 'info' | 'warning';

function actionColor(action: AuditLogEntry['action']): ChipColor {
  if (action === 'auth.login_failed') return 'error';
  if (action.endsWith('.delete') || action === 'token.revoke') return 'warning';
  if (action.endsWith('.create')) return 'success';
  if (action.endsWith('.update')) return 'info';
  return 'default';
}

/** A one-line summary of the entry's action-specific details. */
function describeDetails(e: AuditLogEntry, t: TFunction, locale: string): string {
  const d = e.details;
  const parts: string[] = [];
  if (typeof d.method === 'string') {
    parts.push(t(`auditLog.details.method.${d.method}`, { defaultValue: d.method }));
  }
  if (typeof d.isAdmin === 'boolean') {
    parts.push(d.isAdmin ? t('auditLog.details.admin') : t('auditLog.details.notAdmin'));
  }
  if (d.passwordChanged === true) parts.push(t('auditLog.details.passwordChanged'));
  if (typeof d.username === 'string') {
    parts.push(t('auditLog.details.forUser', { name: d.username }));
  }
  if (typeof d.expiresAt === 'string') {
    parts.push(t('auditLog.details.expiresAt', { date: fmtDate(d.expiresAt, locale) }));
  }
  if (Array.isArray(d.changed)) {
    const fields = d.changed.map((f) =>
      t(`auditLog.details.fields.${String(f)}`, { defaultValue: String(f) }),
    );
    parts.push(t('auditLog.details.changed', { fields: fields.join(', ') }));
  }
  return parts.join(' · ');
}

export default function AuditLogListCard({
  entries,
  loading,
}: {
  entries: AuditLogEntry[];
  loading: boolean;
}) {
  const { t, i18n } = useTranslation();
  return (
    <TableContainer component={Paper}>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('auditLog.time')}</TableCell>
            <TableCell>{t('auditLog.user')}</TableCell>
            <TableCell>{t('auditLog.action')}</TableCell>
            <TableCell>{t('auditLog.target')}</TableCell>
            <TableCell>{t('auditLog.detailsColumn')}</TableCell>
            <TableCell>{t('auditLog.address')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {entries.map((e) => (
            <TableRow key={e.id} hover>
              <TableCell className="audit-log-row__nowrap">
                {fmtDate(e.createdAt, i18n.language)}
              </TableCell>
              <TableCell>
                {e.actorName || '–'}
                {e.actorName && !e.actor && e.action !== 'auth.login_failed' && (
                  <Typography component="span" variant="body2" color="text.secondary">
                    {' '}
                    ({t('auditLog.deletedUser')})
                  </Typography>
                )}
              </TableCell>
              <TableCell>
                <Chip
                  size="small"
                  variant="outlined"
                  color={actionColor(e.action)}
                  label={t(`auditLog.actions.${e.action}`, { defaultValue: e.action })}
                />
              </TableCell>
              <TableCell>
                {e.targetType ? (
                  <>
                    <Typography component="span" variant="body2" color="text.secondary">
                      {t(`auditLog.targetTypes.${e.targetType}`)}{' '}
                    </Typography>
                    {e.targetName || `#${e.targetId}`}
                  </>
                ) : (
                  '–'
                )}
              </TableCell>
              <TableCell>{describeDetails(e, t, i18n.language) || '–'}</TableCell>
              <TableCell className="audit-log-row__nowrap">{e.remoteAddr || '–'}</TableCell>
            </TableRow>
          ))}
          {entries.length === 0 && (
            <TableRow>
              <TableCell colSpan={6} align="center">
                {loading ? t('common.loading') : t('auditLog.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
