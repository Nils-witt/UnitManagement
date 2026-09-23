import { Button, Stack, TableCell, TableRow, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import UnitSymbolIcon from '../../components/UnitSymbolIcon';
import { fmtDate } from '../../lib/format';
import { formatTacticalName } from '../../lib/tacticalName';
import './UnitRow.scss';

export default function UnitRow({
  u,
  onEdit,
  onDelete,
}: {
  u: Unit;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { t, i18n } = useTranslation();
  const p = u.position;
  const tacticalName = formatTacticalName(u.tacticalName);
  return (
    <TableRow hover>
      <TableCell>
        <Stack direction="row" spacing={1.5} className="unit-row__name">
          <UnitSymbolIcon symbol={u.symbol} />
          <div>
            {u.name}
            {tacticalName && (
              <Typography variant="caption" color="text.secondary" component="div">
                {tacticalName}
              </Typography>
            )}
          </div>
        </Stack>
      </TableCell>
      <TableCell>
        {p ? (
          <>
            {p.lat.toFixed(6)}, {p.lon.toFixed(6)}
            {p.height != null && ` · ${t('unitRow.height', { height: p.height.toFixed(1) })}`}
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
          {t('unitRow.by', { name: u.updatedBy?.username ?? t('unitRow.deletedUser') })}
        </Typography>
      </TableCell>
      <TableCell>
        {fmtDate(u.createdAt, i18n.language)}
        <Typography variant="caption" color="text.secondary" component="div">
          {t('unitRow.by', { name: u.createdBy?.username ?? t('unitRow.deletedUser') })}
        </Typography>
      </TableCell>
      <TableCell className="unit-row__actions">
        <Stack direction="row" spacing={1} useFlexGap className="unit-row__action-buttons">
          <Button
            size="small"
            onClick={onEdit}
            aria-label={t('unitRow.editAria', { name: u.name })}
          >
            {t('common.edit')}
          </Button>
          <Button
            size="small"
            color="error"
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
