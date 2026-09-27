import { Button, Chip, Stack, TableCell, TableRow, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import UnitSymbolIcon from '../../components/UnitSymbolIcon';
import { fmtDate, fmtSpeed } from '../../lib/format';
import { formatTacticalName } from '../../lib/tacticalName';
import './UnitRow.scss';

export default function UnitRow({
  u,
  onEdit,
  onHistory,
  onDelete,
}: {
  u: Unit;
  onEdit: () => void;
  onHistory: () => void;
  onDelete: () => void;
}) {
  const { t, i18n } = useTranslation();
  const p = u.position;
  // Synced units have no local creator or editor; the remote's aren't mirrored.
  const byName = (username: string | undefined) =>
    username ??
    (u.syncedFrom ? t('unitRow.syncUser', { name: u.syncedFrom.name }) : t('unitRow.deletedUser'));
  const tacticalName = formatTacticalName(u.tacticalName);
  const synced = u.syncedFrom !== null;
  const syncedTitle = synced ? t('unitRow.syncedHint', { name: u.syncedFrom?.name }) : undefined;
  return (
    <TableRow hover>
      <TableCell>
        <Stack direction="row" spacing={1.5} className="unit-row__name">
          <UnitSymbolIcon symbol={u.symbol} tacticalName={u.tacticalName} />
          <div>
            {u.name}
            {tacticalName && (
              <Typography variant="caption" color="text.secondary" component="div">
                {tacticalName}
              </Typography>
            )}
            {u.syncedFrom && (
              <Chip
                size="small"
                variant="outlined"
                color="info"
                title={syncedTitle}
                label={t('unitRow.syncedFrom', { name: u.syncedFrom.name })}
                className="unit-row__synced"
              />
            )}
          </div>
        </Stack>
      </TableCell>
      <TableCell>
        {p ? (
          <>
            {p.lat.toFixed(6)}, {p.lon.toFixed(6)}
            {p.height != null && ` · ${t('unitRow.height', { height: p.height.toFixed(1) })}`}
            {p.accuracy != null &&
              ` · ${t('unitRow.accuracy', { accuracy: p.accuracy.toFixed(1) })}`}
            {p.speed != null && ` · ${t('unitRow.speed', { speed: fmtSpeed(p.speed) })}`}
            {p.course != null && ` · ${t('unitRow.course', { course: p.course.toFixed(0) })}`}
            <Typography variant="caption" color="text.secondary" component="div">
              {fmtDate(p.timestamp, i18n.language)}
            </Typography>
          </>
        ) : (
          <Typography variant="body2" color="text.secondary" component="span">
            {t('unitRow.noPosition')}
          </Typography>
        )}
      </TableCell>
      <TableCell>
        {fmtDate(u.updatedAt, i18n.language)}
        <Typography variant="caption" color="text.secondary" component="div">
          {t('unitRow.by', { name: byName(u.updatedBy?.username) })}
        </Typography>
      </TableCell>
      <TableCell>
        {fmtDate(u.createdAt, i18n.language)}
        <Typography variant="caption" color="text.secondary" component="div">
          {t('unitRow.by', { name: byName(u.createdBy?.username) })}
        </Typography>
      </TableCell>
      <TableCell className="unit-row__actions">
        <Stack direction="row" spacing={1} useFlexGap className="unit-row__action-buttons">
          <Button
            size="small"
            onClick={onEdit}
            disabled={synced}
            title={syncedTitle}
            aria-label={t('unitRow.editAria', { name: u.name })}
          >
            {t('common.edit')}
          </Button>
          <Button
            size="small"
            onClick={onHistory}
            aria-label={t('unitRow.historyAria', { name: u.name })}
          >
            {t('unitRow.history')}
          </Button>
          <Button
            size="small"
            color="error"
            disabled={synced}
            title={syncedTitle}
            aria-label={t('unitRow.deleteAria', { name: u.name })}
            onClick={onDelete}
          >
            {t('common.delete')}
          </Button>
        </Stack>
      </TableCell>
    </TableRow>
  );
}
