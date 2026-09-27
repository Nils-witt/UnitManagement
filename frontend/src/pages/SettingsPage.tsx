import { type FormEvent, useState } from 'react';
import { Alert, Button, Paper, TextField, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import ErrorBanner from '../components/ErrorBanner';
import { useSettings } from '../hooks/useSettings';
import { errorMessage } from '../lib/errors';
import './settings-page/SettingsPage.scss';

/** Instance-wide settings, stored on the server and applied for everyone. */
export default function SettingsPage() {
  const { t } = useTranslation();
  const { settings, loading, error: loadError, saveSettings } = useSettings();
  // Null until edited, so the field shows the stored value once it loads.
  const [draft, setDraft] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mapStyleUrl = draft ?? settings.mapStyleUrl;

  const edit = (value: string) => {
    setDraft(value);
    setSaved(false);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      await saveSettings({ mapStyleUrl: mapStyleUrl.trim() });
      setDraft(null);
      setSaved(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Paper component="form" className="settings-page" onSubmit={submit}>
      <Typography variant="h6" component="h1">
        {t('settings.mapSection')}
      </Typography>
      <ErrorBanner message={error ?? loadError} />
      {saved && <Alert severity="success">{t('settings.saved')}</Alert>}
      <TextField
        label={t('settings.mapStyleUrl')}
        placeholder="https://example.com/style.json"
        helperText={t('settings.mapStyleUrlHelp')}
        type="url"
        fullWidth
        disabled={loading}
        value={mapStyleUrl}
        onChange={(e) => edit(e.target.value)}
      />
      <div className="settings-page__actions">
        <Button disabled={loading || saving || mapStyleUrl === ''} onClick={() => edit('')}>
          {t('settings.useDefault')}
        </Button>
        <Button
          type="submit"
          variant="contained"
          disabled={loading || saving || mapStyleUrl.trim() === settings.mapStyleUrl}
        >
          {t('common.save')}
        </Button>
      </div>
    </Paper>
  );
}
