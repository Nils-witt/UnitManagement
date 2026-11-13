import { Chip, Stack, Typography } from '@mui/material';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import { useApi } from '../../hooks/useApi';
import { useConfirm } from '../../hooks/useConfirm';
import { errorMessage } from '../../lib/errors';
import { fmtDate } from '../../lib/format';

/** The unit's pending FMS 0, 5 and 9 requests, which leave its status alone.
 * Clicking one acknowledges it; the unit event stream then brings the change
 * into the list. A synced unit's is acknowledged at its remote. */
export default function UnitRequestChips({ unit }: { unit: Unit }) {
  const { t, i18n } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const [busy, setBusy] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pending = unit.requests.filter((r) => r.acknowledgedAt === null);
  if (pending.length === 0) return null;

  const acknowledge = async (code: number) => {
    const ok = await confirm({
      title: t('unitRequests.acknowledgeTitle'),
      message: t('unitRequests.acknowledgeMessage', {
        name: unit.name,
        code,
        label: t(`unitStatus.codes.${code}`),
      }),
      confirmLabel: t('unitRequests.acknowledge'),
    });
    if (!ok) return;
    setBusy(code);
    setError(null);
    try {
      await api.acknowledgeUnitStatusRequest(unit.id, code);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  return (
    <>
      <Stack direction="row" spacing={0.5} useFlexGap className="unit-row__requests">
        {pending.map((r) => (
          <Chip
            key={r.code}
            size="small"
            color={r.code === 0 ? 'error' : 'warning'}
            variant={r.code === 0 ? 'filled' : 'outlined'}
            label={t('unitRequests.chip', { code: r.code })}
            title={t('unitRequests.chipHint', {
              label: t(`unitStatus.codes.${r.code}`),
              at: fmtDate(r.timestamp, i18n.language),
            })}
            aria-label={t('unitRequests.acknowledgeAria', { code: r.code, name: unit.name })}
            disabled={busy !== null}
            onClick={() => void acknowledge(r.code)}
          />
        ))}
      </Stack>
      {error && (
        <Typography variant="caption" color="error" component="div">
          {error}
        </Typography>
      )}
    </>
  );
}
