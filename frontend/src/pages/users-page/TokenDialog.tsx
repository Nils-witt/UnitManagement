import { type FormEvent, useState } from 'react';
import {
  Alert,
  Button,
  IconButton,
  MenuItem,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { useTranslation } from 'react-i18next';
import type { ApiToken, CreatedApiToken, User } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { useApi } from '../../hooks/useApi';
import { useConfirm } from '../../hooks/useConfirm';
import { useUserTokens } from '../../hooks/useUserTokens';
import { errorMessage } from '../../lib/errors';
import { fmtDate } from '../../lib/format';
import './TokenDialog.scss';

const UNIT_SECONDS = { hours: 3600, days: 86400 } as const;
type TtlUnit = keyof typeof UNIT_SECONDS;

/** Match the server's bounds (auth.MinTokenTTL, auth.MaxTokenTTL,
 * auth.MaxTokenNameLength). */
const MIN_TTL_SECONDS = 60;
const MAX_TTL_SECONDS = 10 * 365 * 86400;
const MAX_NAME_LENGTH = 64;

/** Shows a just-created token's value, which the server won't return again. */
function CreatedToken({ created }: { created: CreatedApiToken }) {
  const { t, i18n } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(created.token);
      setCopied(true);
    } catch {
      setError(t('tokenDialog.copyFailed'));
    }
  };

  return (
    <Stack spacing={2}>
      <ErrorBanner message={error} />
      <Alert severity="warning">{t('tokenDialog.shownOnce', { name: created.name })}</Alert>
      <Stack direction="row" spacing={1} className="token-dialog__token">
        <TextField
          label={t('tokenDialog.tokenLabel')}
          size="small"
          fullWidth
          value={created.token}
          helperText={t('tokenDialog.expiresAt', {
            date: fmtDate(created.expiresAt, i18n.language),
          })}
          slotProps={{ htmlInput: { readOnly: true, className: 'token-dialog__token-input' } }}
          onFocus={(e) => e.target.select()}
        />
        <Tooltip title={copied ? t('tokenDialog.copied') : t('tokenDialog.copy')}>
          <IconButton aria-label={t('tokenDialog.copy')} onClick={() => void copy()}>
            <ContentCopyIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>
    </Stack>
  );
}

function TokensTable({
  tokens,
  loading,
  onRevoke,
}: {
  tokens: ApiToken[];
  loading: boolean;
  onRevoke: (token: ApiToken) => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <TableContainer>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('tokenDialog.nameLabel')}</TableCell>
            <TableCell>{t('tokenDialog.created')}</TableCell>
            <TableCell>{t('tokenDialog.expires')}</TableCell>
            <TableCell>{t('common.actions')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {tokens.map((tok) => (
            <TableRow key={tok.id} hover>
              <TableCell>{tok.name}</TableCell>
              <TableCell>{fmtDate(tok.createdAt, i18n.language)}</TableCell>
              <TableCell>{fmtDate(tok.expiresAt, i18n.language)}</TableCell>
              <TableCell>
                <Button
                  size="small"
                  color="error"
                  aria-label={t('tokenDialog.revokeAria', { name: tok.name })}
                  onClick={() => onRevoke(tok)}
                >
                  {t('tokenDialog.revoke')}
                </Button>
              </TableCell>
            </TableRow>
          ))}
          {tokens.length === 0 && (
            <TableRow>
              <TableCell colSpan={4} align="center">
                {loading ? t('common.loading') : t('tokenDialog.empty')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableContainer>
  );
}

function CreateTokenForm({
  onCreate,
}: {
  onCreate: (name: string, ttlSeconds: number) => Promise<void>;
}) {
  const { t } = useTranslation();
  const [name, setName] = useState('');
  const [amount, setAmount] = useState('30');
  const [unit, setUnit] = useState<TtlUnit>('days');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const ttlSeconds = Math.round(Number(amount) * UNIT_SECONDS[unit]);
  const ttlInvalid =
    amount.trim() === '' ||
    !Number.isFinite(ttlSeconds) ||
    ttlSeconds < MIN_TTL_SECONDS ||
    ttlSeconds > MAX_TTL_SECONDS;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onCreate(name.trim(), ttlSeconds);
      setName('');
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Stack component="form" spacing={2} onSubmit={handleSubmit}>
      <Typography variant="subtitle2">{t('tokenDialog.newToken')}</Typography>
      <ErrorBanner message={error} />
      <Stack direction="row" spacing={2} useFlexGap className="token-dialog__fields">
        <TextField
          label={t('tokenDialog.nameLabel')}
          size="small"
          required
          autoFocus
          helperText={t('tokenDialog.nameHint')}
          value={name}
          onChange={(e) => setName(e.target.value)}
          slotProps={{ htmlInput: { maxLength: MAX_NAME_LENGTH } }}
        />
        <TextField
          label={t('tokenDialog.ttlLabel')}
          type="number"
          size="small"
          required
          error={ttlInvalid}
          helperText={t('tokenDialog.ttlHint')}
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          slotProps={{ htmlInput: { min: 1, step: 1 } }}
          className="token-dialog__amount"
        />
        <TextField
          select
          label={t('tokenDialog.unitLabel')}
          size="small"
          value={unit}
          onChange={(e) => setUnit(e.target.value as TtlUnit)}
          className="token-dialog__unit"
        >
          <MenuItem value="hours">{t('tokenDialog.hours')}</MenuItem>
          <MenuItem value="days">{t('tokenDialog.days')}</MenuItem>
        </TextField>
      </Stack>
      <Stack direction="row" className="token-dialog__actions">
        <Button
          type="submit"
          variant="contained"
          disabled={submitting || ttlInvalid || !name.trim()}
        >
          {t('common.create')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** Mounted by the modal only while it is open, so it starts fresh each time. */
function TokenManager({ user }: { user: User }) {
  const { t } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const { data: tokens, loading, error: loadError, reload } = useUserTokens(user.id);
  const [created, setCreated] = useState<CreatedApiToken | null>(null);
  const [error, setError] = useState<string | null>(null);

  const onCreate = async (name: string, ttlSeconds: number) => {
    setCreated(await api.createUserToken(user.id, { name, ttlSeconds }));
    await reload();
  };

  const onRevoke = async (token: ApiToken) => {
    const ok = await confirm({
      title: t('tokenDialog.revokeTitle'),
      message: t('tokenDialog.revokeMessage', { name: token.name }),
      confirmLabel: t('tokenDialog.revoke'),
      destructive: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await api.revokeUserToken(user.id, token.id);
      if (created?.id === token.id) setCreated(null);
      await reload();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Stack spacing={3} className="token-dialog__content">
      <Alert severity="info">{t('tokenDialog.info', { name: user.username })}</Alert>
      {created && <CreatedToken key={created.id} created={created} />}
      <ErrorBanner message={error ?? loadError} />
      <TokensTable tokens={tokens} loading={loading} onRevoke={(tok) => void onRevoke(tok)} />
      <CreateTokenForm onCreate={onCreate} />
    </Stack>
  );
}

/** Lists, creates and revokes the API tokens that sign in as `user`. */
export default function TokenDialog({
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
      title={t('tokenDialog.title', { name: user?.username ?? '' })}
      onClose={onClose}
    >
      {user && <TokenManager user={user} />}
    </Modal>
  );
}
