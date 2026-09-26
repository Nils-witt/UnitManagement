import { useState } from 'react';
import { Box, Button, MenuItem, TextField, Typography } from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';
import { useTranslation } from 'react-i18next';
import { AUDIT_ACTIONS, type AuditAction } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useAuditLog } from '../hooks/useAuditLog';
import AuditLogListCard from './audit-log-page/AuditLogListCard';
import './users-page/UsersToolbar.scss';
import './audit-log-page/AuditLogPage.scss';

const ALL = 'all';

/** Who signed in and who changed what, newest first. */
export default function AuditLogPage() {
  const { t } = useTranslation();
  const [action, setAction] = useState<AuditAction | null>(null);
  const { entries, loading, error, hasMore, loadingMore, loadMore, reload } = useAuditLog(action);

  return (
    <Box>
      <div className="users-toolbar">
        <TextField
          select
          label={t('auditLog.actionFilter')}
          size="small"
          className="audit-log-page__filter"
          value={action ?? ALL}
          onChange={(e) =>
            setAction(e.target.value === ALL ? null : (e.target.value as AuditAction))
          }
        >
          <MenuItem value={ALL}>{t('auditLog.allActions')}</MenuItem>
          {AUDIT_ACTIONS.map((a) => (
            <MenuItem key={a} value={a}>
              {t(`auditLog.actions.${a}`)}
            </MenuItem>
          ))}
        </TextField>
        <Typography variant="body2" color="text.secondary" className="audit-log-page__notice">
          {t('auditLog.positionNotice')}
        </Typography>
        <Button startIcon={<RefreshIcon />} onClick={reload} disabled={loading}>
          {t('auditLog.refresh')}
        </Button>
      </div>
      <ErrorBanner message={error} />
      <AuditLogListCard entries={entries} loading={loading} />
      {hasMore && (
        <Box className="audit-log-page__more">
          <Button variant="outlined" onClick={loadMore} disabled={loadingMore}>
            {loadingMore ? t('common.loading') : t('auditLog.loadMore')}
          </Button>
        </Box>
      )}
    </Box>
  );
}
