import {
  Button,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { SyncRemote } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { useSyncRemoteLogs } from '../../hooks/useSync';
import { fmtDate } from '../../lib/format';

/** The remote's recent sync activity, newest first. */
function SyncLogs({ remote }: { remote: SyncRemote }) {
  const { t, i18n } = useTranslation();
  const { data: logs, loading, error, reload } = useSyncRemoteLogs(remote.id);
  const newestFirst = [...logs].reverse();
  return (
    <Stack spacing={2}>
      <Typography variant="body2" color="text.secondary">
        {t('syncLogs.notice')}
      </Typography>
      <ErrorBanner message={error} />
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t('syncLogs.time')}</TableCell>
              <TableCell>{t('syncLogs.message')}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {newestFirst.map((entry, i) => (
              <TableRow key={`${entry.time}-${i}`}>
                <TableCell className="sync-page__nowrap">
                  {fmtDate(entry.time, i18n.language)}
                </TableCell>
                <TableCell>
                  <Typography
                    variant="body2"
                    color={entry.level === 'error' ? 'error' : undefined}
                    className="sync-page__mono"
                  >
                    {entry.message}
                  </Typography>
                </TableCell>
              </TableRow>
            ))}
            {logs.length === 0 && (
              <TableRow>
                <TableCell colSpan={2} align="center">
                  {loading ? t('common.loading') : t('syncLogs.empty')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <Stack direction="row" className="sync-page__form-actions">
        <Button onClick={() => void reload()}>{t('syncPage.refresh')}</Button>
      </Stack>
    </Stack>
  );
}

export default function SyncLogsDialog({
  open,
  remote,
  onClose,
}: {
  open: boolean;
  remote: SyncRemote | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Modal open={open} title={t('syncLogs.title', { name: remote?.name ?? '' })} onClose={onClose}>
      {open && remote && <SyncLogs remote={remote} />}
    </Modal>
  );
}
