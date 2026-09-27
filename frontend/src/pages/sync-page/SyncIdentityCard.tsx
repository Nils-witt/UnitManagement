import { useState } from 'react';
import { IconButton, Paper, Stack, TextField, Tooltip, Typography } from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import { useTranslation } from 'react-i18next';
import ErrorBanner from '../../components/ErrorBanner';
import { useSyncIdentity } from '../../hooks/useSync';

function CopyField({
  label,
  value,
  multiline,
}: {
  label: string;
  value: string;
  multiline?: boolean;
}) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setError(null);
    } catch {
      setError(t('syncPage.copyFailed'));
    }
  };

  return (
    <Stack spacing={1}>
      <ErrorBanner message={error} />
      <Stack direction="row" spacing={1} className="sync-page__copy-field">
        <TextField
          label={label}
          size="small"
          fullWidth
          multiline={multiline}
          value={value}
          slotProps={{ htmlInput: { readOnly: true, className: 'sync-page__mono' } }}
          onFocus={(e) => e.target.select()}
        />
        <Tooltip title={copied ? t('syncPage.copied') : t('syncPage.copy')}>
          <span>
            <IconButton
              aria-label={t('syncPage.copyAria', { label })}
              disabled={!value}
              onClick={() => void copy()}
            >
              <ContentCopyIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Stack>
    </Stack>
  );
}

/** This instance's server UUID and public key, which other instances register
 * as an API key to let it sync from them. */
export default function SyncIdentityCard() {
  const { t } = useTranslation();
  const { data: identity, error } = useSyncIdentity();
  return (
    <Paper className="sync-page__identity">
      <Typography variant="h6" component="h2">
        {t('syncPage.identityTitle')}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t('syncPage.identityHelp')}
      </Typography>
      <ErrorBanner message={error} />
      <CopyField label={t('syncPage.serverUuid')} value={identity.serverUuid} />
      <CopyField label={t('syncPage.publicKey')} value={identity.publicKeyPem.trim()} multiline />
    </Paper>
  );
}
