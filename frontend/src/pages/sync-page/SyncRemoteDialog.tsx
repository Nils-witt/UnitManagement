import { type FormEvent, useState } from 'react';
import { Button, FormControlLabel, Stack, Switch, TextField } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { SyncRemote, SyncRemoteInput } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { errorMessage } from '../../lib/errors';

/** Match the server's bounds (remotesync.MinPollIntervalSec, ...). */
const MIN_INTERVAL = 10;
const MAX_INTERVAL = 24 * 60 * 60;
const MAX_NAME_LENGTH = 64;
const DEFAULT_INTERVAL = 60;

/** Mounted by the modal only while it is open, so it starts fresh each time. */
function SyncRemoteForm({
  remote,
  onClose,
  onSubmit,
}: {
  remote: SyncRemote | null;
  onClose: () => void;
  onSubmit: (input: SyncRemoteInput) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState(remote?.name ?? '');
  const [baseUrl, setBaseUrl] = useState(remote?.baseUrl ?? '');
  const [intervalText, setIntervalText] = useState(
    String(remote?.pollIntervalSec ?? DEFAULT_INTERVAL),
  );
  const [enabled, setEnabled] = useState(remote?.enabled ?? true);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const intervalSec = Number(intervalText);
  const intervalInvalid =
    !Number.isInteger(intervalSec) || intervalSec < MIN_INTERVAL || intervalSec > MAX_INTERVAL;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit({
        name: name.trim(),
        baseUrl: baseUrl.trim(),
        pollIntervalSec: intervalSec,
        enabled,
      });
      onClose();
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  };

  return (
    <Stack component="form" spacing={3} className="sync-page__form" onSubmit={handleSubmit}>
      <ErrorBanner message={error} />
      <TextField
        label={t('syncRemoteDialog.nameLabel')}
        size="small"
        required
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
        slotProps={{ htmlInput: { maxLength: MAX_NAME_LENGTH } }}
      />
      <TextField
        label={t('syncRemoteDialog.baseUrlLabel')}
        placeholder="https://units.example.com"
        helperText={t('syncRemoteDialog.baseUrlHint')}
        type="url"
        size="small"
        required
        value={baseUrl}
        onChange={(e) => setBaseUrl(e.target.value)}
      />
      <TextField
        label={t('syncRemoteDialog.intervalLabel')}
        helperText={t('syncRemoteDialog.intervalHint')}
        type="number"
        size="small"
        required
        error={intervalInvalid}
        value={intervalText}
        onChange={(e) => setIntervalText(e.target.value)}
        slotProps={{ htmlInput: { min: MIN_INTERVAL, max: MAX_INTERVAL, step: 1 } }}
      />
      <FormControlLabel
        control={<Switch checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />}
        label={t('syncRemoteDialog.enabled')}
      />
      <Stack direction="row" spacing={1} className="sync-page__form-actions">
        <Button onClick={onClose} disabled={submitting}>
          {t('common.cancel')}
        </Button>
        <Button
          type="submit"
          variant="contained"
          disabled={submitting || intervalInvalid || !name.trim() || !baseUrl.trim()}
        >
          {remote ? t('common.save') : t('common.create')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** Creates a sync remote (`remote` is null) or edits an existing one. */
export default function SyncRemoteDialog({
  open,
  remote,
  onClose,
  onSubmit,
}: {
  open: boolean;
  remote: SyncRemote | null;
  onClose: () => void;
  onSubmit: (input: SyncRemoteInput) => Promise<void>;
}) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      title={
        remote
          ? t('syncRemoteDialog.editTitle', { name: remote.name })
          : t('syncRemoteDialog.newTitle')
      }
      onClose={onClose}
    >
      <SyncRemoteForm remote={remote} onClose={onClose} onSubmit={onSubmit} />
    </Modal>
  );
}
