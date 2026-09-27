import { type FormEvent, useState } from 'react';
import {
  Alert,
  Button,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { ApiKey, CreateApiKeyInput, User } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { useApi } from '../../hooks/useApi';
import { useConfirm } from '../../hooks/useConfirm';
import { useUserApiKeys } from '../../hooks/useUserApiKeys';
import { errorMessage } from '../../lib/errors';
import { fmtDate } from '../../lib/format';
import './ApiKeyDialog.scss';

/** Matches the server's auth.MaxAPIKeyNameLength. */
const MAX_NAME_LENGTH = 64;
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function ApiKeysTable({
  keys,
  loading,
  onDelete,
}: {
  keys: ApiKey[];
  loading: boolean;
  onDelete: (key: ApiKey) => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('apiKeyDialog.nameLabel')}</TableCell>
            <TableCell>{t('apiKeyDialog.idLabel')}</TableCell>
            <TableCell>{t('apiKeyDialog.created')}</TableCell>
            <TableCell>{t('apiKeyDialog.lastUsed')}</TableCell>
            <TableCell>{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {keys.map((k) => (
            <TableRow key={k.id} hover>
              <TableCell>{k.name}</TableCell>
              <TableCell className="api-key-dialog__mono">{k.id}</TableCell>
              <TableCell>{fmtDate(k.createdAt, i18n.language)}</TableCell>
              <TableCell>
                {k.lastUsedAt ? fmtDate(k.lastUsedAt, i18n.language) : t('apiKeyDialog.never')}
              </TableCell>
              <TableCell>
                <Button
                  size="small"
                  color="error"
                  aria-label={t('apiKeyDialog.deleteAria', { name: k.name })}
                  onClick={() => onDelete(k)}
                >
                  {t('common.delete')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {keys.length === 0 && (
            <TableRow>
              <TableCell colSpan={5} align="center">
                {loading ? t('common.loading') : t('apiKeyDialog.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function CreateApiKeyForm({ onCreate }: { onCreate: (input: CreateApiKeyInput) => Promise<void> }) {
  const { t } = useTranslation();
  const [id, setId] = useState('');
  const [name, setName] = useState('');
  const [publicKeyPem, setPublicKeyPem] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const idInvalid = id.trim() !== '' && !UUID_PATTERN.test(id.trim());

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onCreate({ id: id.trim(), name: name.trim(), publicKeyPem: publicKeyPem.trim() });
      setId('');
      setName('');
      setPublicKeyPem('');
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Stack component="form" spacing={2} onSubmit={handleSubmit}>
      <Typography variant="subtitle2">{t('apiKeyDialog.newKey')}</Typography>
      <ErrorBanner message={error} />
      <Stack direction="row" spacing={2} useFlexGap className="api-key-dialog__fields">
        <TextField
          label={t('apiKeyDialog.nameLabel')}
          size="small"
          required
          helperText={t('apiKeyDialog.nameHint')}
          value={name}
          onChange={(e) => setName(e.target.value)}
          slotProps={{ htmlInput: { maxLength: MAX_NAME_LENGTH } }}
        />
        <TextField
          label={t('apiKeyDialog.idLabel')}
          size="small"
          required
          error={idInvalid}
          helperText={t('apiKeyDialog.idHint')}
          value={id}
          onChange={(e) => setId(e.target.value)}
          className="api-key-dialog__id"
          slotProps={{ htmlInput: { className: 'api-key-dialog__mono' } }}
        />
      </Stack>
      <TextField
        label={t('apiKeyDialog.publicKeyLabel')}
        size="small"
        required
        multiline
        minRows={4}
        placeholder="-----BEGIN PUBLIC KEY-----"
        helperText={t('apiKeyDialog.publicKeyHint')}
        value={publicKeyPem}
        onChange={(e) => setPublicKeyPem(e.target.value)}
        slotProps={{ htmlInput: { className: 'api-key-dialog__mono' } }}
      />
      <Stack direction="row" className="api-key-dialog__actions">
        <Button
          type="submit"
          variant="contained"
          disabled={submitting || idInvalid || !id.trim() || !name.trim() || !publicKeyPem.trim()}
        >
          {t('common.create')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** Mounted by the modal only while it is open, so it starts fresh each time. */
function ApiKeyManager({ user }: { user: User }) {
  const { t } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const { data: keys, loading, error: loadError, reload } = useUserApiKeys(user.id);
  const [error, setError] = useState<string | null>(null);

  const onCreate = async (input: CreateApiKeyInput) => {
    await api.createUserApiKey(user.id, input);
    await reload();
  };

  const onDelete = async (key: ApiKey) => {
    const ok = await confirm({
      title: t('apiKeyDialog.deleteTitle'),
      message: t('apiKeyDialog.deleteMessage', { name: key.name }),
      confirmLabel: t('common.delete'),
      destructive: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await api.deleteUserApiKey(user.id, key.id);
      await reload();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Stack spacing={3} className="api-key-dialog__content">
      <Alert severity="info">{t('apiKeyDialog.info', { name: user.username })}</Alert>
      <ErrorBanner message={error ?? loadError} />
      <ApiKeysTable keys={keys} loading={loading} onDelete={(k) => void onDelete(k)} />
      <CreateApiKeyForm onCreate={onCreate} />
    </Stack>
  );
}

/** Lists, registers and deletes the public keys that act as `user`. */
export default function ApiKeyDialog({
  open,
  user,
  onClose,
}: {
  open: boolean;
  user: User | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      title={t('apiKeyDialog.title', { name: user?.username ?? '' })}
      onClose={onClose}
    >
      {user && <ApiKeyManager user={user} />}
    </Modal>
  );
}
