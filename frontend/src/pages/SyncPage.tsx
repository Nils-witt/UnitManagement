import { useEffect, useState } from 'react';
import {
  Box,
  Button,
  Chip,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import { useTranslation } from 'react-i18next';
import type { SyncRemote, SyncRemoteInput } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useApi } from '../hooks/useApi';
import { useConfirm } from '../hooks/useConfirm';
import { useSyncRemotes } from '../hooks/useSync';
import { errorMessage } from '../lib/errors';
import { fmtDate } from '../lib/format';
import SyncIdentityCard from './sync-page/SyncIdentityCard';
import SyncLogsDialog from './sync-page/SyncLogsDialog';
import SyncRemoteDialog from './sync-page/SyncRemoteDialog';
import './sync-page/SyncPage.scss';

/** How often the remotes' sync status is refreshed while the page is open. */
const STATUS_REFRESH_MS = 10_000;

function StatusChip({ remote }: { remote: SyncRemote }) {
  const { t } = useTranslation();
  if (!remote.enabled) {
    return <Chip size="small" variant="outlined" label={t('syncPage.statusDisabled')} />;
  }
  if (remote.lastSyncStatus === 'ok') {
    return <Chip size="small" color="success" variant="outlined" label={t('syncPage.statusOk')} />;
  }
  if (remote.lastSyncStatus === 'error') {
    return (
      <Chip
        size="small"
        color="error"
        variant="outlined"
        title={remote.lastSyncError}
        label={t('syncPage.statusError')}
      />
    );
  }
  return <Chip size="small" variant="outlined" label={t('syncPage.statusPending')} />;
}

/** Other instances whose units are mirrored here, and this instance's
 * identity for letting it sync from them. */
export default function SyncPage() {
  const { t, i18n } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const { data: remotes, loading, error: loadError, reload } = useSyncRemotes();
  // The edited remote stays set while the dialog closes, so its form doesn't
  // flip to "new remote" during the exit animation.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<SyncRemote | null>(null);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logsRemote, setLogsRemote] = useState<SyncRemote | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const id = window.setInterval(() => void reload(), STATUS_REFRESH_MS);
    return () => window.clearInterval(id);
  }, [reload]);

  const openDialog = (remote: SyncRemote | null) => {
    setEditing(remote);
    setDialogOpen(true);
  };

  const onSubmit = async (input: SyncRemoteInput) => {
    if (editing) {
      await api.updateSyncRemote(editing.id, input);
    } else {
      await api.createSyncRemote(input);
    }
    await reload();
  };

  const run = async (action: () => Promise<void>) => {
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const onTrigger = (remote: SyncRemote) =>
    run(async () => {
      await api.triggerSyncRemote(remote.id);
      // The status changes once the sync is done, usually within a moment.
      window.setTimeout(() => void reload(), 1500);
    });

  const onDelete = async (remote: SyncRemote) => {
    const ok = await confirm({
      title: t('syncPage.deleteTitle'),
      message: t('syncPage.deleteMessage', { name: remote.name }),
      confirmLabel: t('common.delete'),
      destructive: true,
    });
    if (!ok) return;
    await run(async () => {
      await api.deleteSyncRemote(remote.id);
      await reload();
    });
  };

  return (
    <Box className="sync-page">
      <SyncIdentityCard />
      <div className="sync-page__toolbar">
        <Typography variant="h6" component="h2">
          {t('syncPage.remotesTitle')}
        </Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => openDialog(null)}>
          {t('syncPage.newRemote')}
        </Button>
      </div>
      <Typography variant="body2" color="text.secondary">
        {t('syncPage.remotesHelp')}
      </Typography>
      <ErrorBanner message={error ?? loadError} />
      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t('syncPage.name')}</TableCell>
              <TableCell>{t('syncPage.baseUrl')}</TableCell>
              <TableCell>{t('syncPage.interval')}</TableCell>
              <TableCell>{t('syncPage.status')}</TableCell>
              <TableCell>{t('common.actions')}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {remotes.map((r) => (
              <TableRow key={r.id} hover>
                <TableCell>{r.name}</TableCell>
                <TableCell className="sync-page__mono">{r.baseUrl}</TableCell>
                <TableCell>{t('syncPage.intervalValue', { count: r.pollIntervalSec })}</TableCell>
                <TableCell>
                  <StatusChip remote={r} />
                  {r.lastSyncAt && (
                    <Typography variant="caption" color="text.secondary" component="div">
                      {fmtDate(r.lastSyncAt, i18n.language)}
                    </Typography>
                  )}
                  {r.enabled && r.lastSyncStatus === 'error' && (
                    <Typography variant="caption" color="error" component="div">
                      {r.lastSyncError}
                    </Typography>
                  )}
                </TableCell>
                <TableCell>
                  <Stack direction="row" spacing={1} useFlexGap className="sync-page__actions">
                    <Button
                      size="small"
                      disabled={!r.enabled}
                      aria-label={t('syncPage.syncNowAria', { name: r.name })}
                      onClick={() => void onTrigger(r)}
                    >
                      {t('syncPage.syncNow')}
                    </Button>
                    <Button
                      size="small"
                      aria-label={t('syncPage.logsAria', { name: r.name })}
                      onClick={() => {
                        setLogsRemote(r);
                        setLogsOpen(true);
                      }}
                    >
                      {t('syncPage.logs')}
                    </Button>
                    <Button
                      size="small"
                      aria-label={t('syncPage.editAria', { name: r.name })}
                      onClick={() => openDialog(r)}
                    >
                      {t('common.edit')}
                    </Button>
                    <Button
                      size="small"
                      color="error"
                      aria-label={t('syncPage.deleteAria', { name: r.name })}
                      onClick={() => void onDelete(r)}
                    >
                      {t('common.delete')}
                    </Button>
                  </Stack>
                </TableCell>
              </TableRow>
            ))}
            {remotes.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} align="center">
                  {loading ? t('common.loading') : t('syncPage.empty')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
      <SyncRemoteDialog
        open={dialogOpen}
        remote={editing}
        onClose={() => setDialogOpen(false)}
        onSubmit={onSubmit}
      />
      <SyncLogsDialog open={logsOpen} remote={logsRemote} onClose={() => setLogsOpen(false)} />
    </Box>
  );
}
