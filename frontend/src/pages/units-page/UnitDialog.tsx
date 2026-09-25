import { type FormEvent, useState } from 'react';
import { Button, FormControlLabel, Stack, Switch, TextField, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Unit, UnitInput } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { errorMessage } from '../../lib/errors';
import { cleanTacticalName } from '../../lib/tacticalName';
import { cleanSymbol } from '../../lib/unitSymbol';
import TacticalNameFields from './TacticalNameFields';
import UnitSymbolFields from './UnitSymbolFields';
import './UnitDialog.scss';

/** Parses a decimal typed with either "." or "," as separator; NaN if it isn't one. */
function parseDecimal(value: string): number {
  const v = value.trim().replace(',', '.');
  return v === '' ? NaN : Number(v);
}

function inRange(value: number, limit: number): boolean {
  return Number.isFinite(value) && value >= -limit && value <= limit;
}

/** Mounted by the modal only while it is open, so it starts fresh each time. */
function UnitForm({
  unit,
  onClose,
  onSubmit,
}: {
  unit: Unit | null;
  onClose: () => void;
  onSubmit: (input: UnitInput) => Promise<void>;
}) {
  const { t } = useTranslation();
  const editing = unit != null;
  const initial = unit?.position;
  const [name, setName] = useState(unit?.name ?? '');
  const [hasPosition, setHasPosition] = useState(initial != null);
  const [lat, setLat] = useState(initial ? String(initial.lat) : '');
  const [lon, setLon] = useState(initial ? String(initial.lon) : '');
  const [height, setHeight] = useState(initial?.height != null ? String(initial.height) : '');
  const [accuracy, setAccuracy] = useState(
    initial?.accuracy != null ? String(initial.accuracy) : '',
  );
  const [symbol, setSymbol] = useState(unit?.symbol ?? {});
  const [tacticalName, setTacticalName] = useState(unit?.tacticalName ?? {});
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const latValue = parseDecimal(lat);
  const lonValue = parseDecimal(lon);
  const heightValue = height.trim() === '' ? null : parseDecimal(height);
  const latInvalid = hasPosition && lat.trim() !== '' && !inRange(latValue, 90);
  const lonInvalid = hasPosition && lon.trim() !== '' && !inRange(lonValue, 180);
  const heightInvalid = hasPosition && heightValue !== null && !Number.isFinite(heightValue);
  const accuracyValue = accuracy.trim() === '' ? null : parseDecimal(accuracy);
  const accuracyInvalid =
    hasPosition &&
    accuracyValue !== null &&
    !(Number.isFinite(accuracyValue) && accuracyValue >= 0);
  const positionIncomplete =
    hasPosition &&
    (!inRange(latValue, 90) || !inRange(lonValue, 180) || heightInvalid || accuracyInvalid);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    let position: UnitInput['position'] = null;
    if (hasPosition) {
      position = { lat: latValue, lon: lonValue, height: heightValue, accuracy: accuracyValue };
      // An unchanged position keeps when it was measured; a new one is
      // stamped with the current time by the server.
      if (
        initial &&
        initial.lat === latValue &&
        initial.lon === lonValue &&
        initial.height === heightValue &&
        initial.accuracy === accuracyValue
      ) {
        position.timestamp = initial.timestamp;
      }
    }
    try {
      await onSubmit({
        name: name.trim(),
        position,
        symbol: cleanSymbol(symbol),
        tacticalName: cleanTacticalName(tacticalName),
      });
      onClose();
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  };

  return (
    <Stack component="form" spacing={3} className="unit-dialog__form" onSubmit={handleSubmit}>
      <ErrorBanner message={error} />
      <TextField
        label={t('unitDialog.nameLabel')}
        size="small"
        required
        autoFocus
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      <Stack spacing={1.5}>
        <Typography variant="subtitle2">{t('tacticalName.title')}</Typography>
        <TacticalNameFields value={tacticalName} onChange={setTacticalName} />
      </Stack>
      <Stack spacing={1.5}>
        <Typography variant="subtitle2">{t('unitSymbol.title')}</Typography>
        <UnitSymbolFields value={symbol} tacticalName={tacticalName} onChange={setSymbol} />
      </Stack>
      <FormControlLabel
        control={
          <Switch checked={hasPosition} onChange={(e) => setHasPosition(e.target.checked)} />
        }
        label={t('unitDialog.hasPosition')}
      />
      {hasPosition && (
        <Stack direction="row" spacing={2} useFlexGap className="unit-dialog__coordinates">
          <TextField
            label={t('unitDialog.latLabel')}
            size="small"
            required
            slotProps={{ htmlInput: { inputMode: 'decimal' } }}
            error={latInvalid}
            helperText={t('unitDialog.latHint')}
            value={lat}
            onChange={(e) => setLat(e.target.value)}
          />
          <TextField
            label={t('unitDialog.lonLabel')}
            size="small"
            required
            slotProps={{ htmlInput: { inputMode: 'decimal' } }}
            error={lonInvalid}
            helperText={t('unitDialog.lonHint')}
            value={lon}
            onChange={(e) => setLon(e.target.value)}
          />
          <TextField
            label={t('unitDialog.heightLabel')}
            size="small"
            slotProps={{ htmlInput: { inputMode: 'decimal' } }}
            error={heightInvalid}
            helperText={t('unitDialog.heightHint')}
            value={height}
            onChange={(e) => setHeight(e.target.value)}
          />
          <TextField
            label={t('unitDialog.accuracyLabel')}
            size="small"
            slotProps={{ htmlInput: { inputMode: 'decimal' } }}
            error={accuracyInvalid}
            helperText={t('unitDialog.accuracyHint')}
            value={accuracy}
            onChange={(e) => setAccuracy(e.target.value)}
          />
        </Stack>
      )}
      <Stack direction="row" spacing={1} className="unit-dialog__actions">
        <Button onClick={onClose} disabled={submitting}>
          {t('common.cancel')}
        </Button>
        <Button
          type="submit"
          variant="contained"
          disabled={submitting || !name.trim() || positionIncomplete}
        >
          {editing ? t('common.save') : t('common.create')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** Creates a unit (`unit` is null) or edits an existing one. */
export default function UnitDialog({
  open,
  unit,
  onClose,
  onSubmit,
}: {
  open: boolean;
  unit: Unit | null;
  onClose: () => void;
  onSubmit: (input: UnitInput) => Promise<void>;
}) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      title={unit ? t('unitDialog.editTitle', { name: unit.name }) : t('unitDialog.newTitle')}
      onClose={onClose}
    >
      <UnitForm unit={unit} onClose={onClose} onSubmit={onSubmit} />
    </Modal>
  );
}
