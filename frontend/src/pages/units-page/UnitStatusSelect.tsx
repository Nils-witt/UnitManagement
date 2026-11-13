import { MenuItem, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import { useApi } from '../../hooks/useApi';
import { errorMessage } from '../../lib/errors';
import { fmsStatusCodes } from '../../lib/fmsStatus';

const none = 'none';

/** Sets a unit's FMS status right away. The unit event stream brings the
 * change into the list, so nothing is reloaded here. */
export default function UnitStatusSelect({ unit, disabled }: { unit: Unit; disabled: boolean }) {
  const { t } = useTranslation();
  const api = useApi();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onChange = async (value: string) => {
    setSaving(true);
    setError(null);
    try {
      if (value === none) await api.clearUnitStatus(unit.id);
      else await api.setUnitStatus(unit.id, Number(value));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <TextField
        select
        size="small"
        value={unit.status ? String(unit.status.code) : none}
        onChange={(e) => void onChange(e.target.value)}
        disabled={disabled || saving}
        slotProps={{ htmlInput: { 'aria-label': t('unitStatus.setAria', { name: unit.name }) } }}
        sx={{ minWidth: 200 }}
      >
        <MenuItem value={none}>{t('unitStatus.none')}</MenuItem>
        {fmsStatusCodes.map((code) => (
          <MenuItem key={code} value={String(code)}>
            {code} – {t(`unitStatus.codes.${code}`)}
          </MenuItem>
        ))}
      </TextField>
      {error && (
        <Typography variant="caption" color="error" component="div">
          {error}
        </Typography>
      )}
    </>
  );
}
