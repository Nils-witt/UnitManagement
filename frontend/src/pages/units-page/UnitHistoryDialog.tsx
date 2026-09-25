import {
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { Unit } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { useUnitPositions } from '../../hooks/useUnitPositions';
import { fmtDate } from '../../lib/format';

/** Mounted by the modal only while it is open, so it fetches fresh each time. */
function HistoryTable({ unit }: { unit: Unit }) {
  const { t, i18n } = useTranslation();
  const { data: history, loading, error } = useUnitPositions(unit.id);
  return (
    <>
      <ErrorBanner message={error} />
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t('unitHistory.measured')}</TableCell>
              <TableCell>{t('unitsListCard.position')}</TableCell>
              <TableCell>{t('unitHistory.height')}</TableCell>
              <TableCell>{t('unitHistory.accuracy')}</TableCell>
              <TableCell>{t('unitHistory.recorded')}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {history.map((p, i) => (
              <TableRow key={`${p.recordedAt}-${i}`} hover>
                <TableCell>{fmtDate(p.timestamp, i18n.language)}</TableCell>
                <TableCell>
                  {p.lat.toFixed(6)}, {p.lon.toFixed(6)}
                </TableCell>
                <TableCell>
                  {p.height != null ? t('unitRow.height', { height: p.height.toFixed(1) }) : '–'}
                </TableCell>
                <TableCell>
                  {p.accuracy != null
                    ? t('unitRow.accuracy', { accuracy: p.accuracy.toFixed(1) })
                    : '–'}
                </TableCell>
                <TableCell>
                  {fmtDate(p.recordedAt, i18n.language)}
                  <Typography variant="caption" color="text.secondary" component="div">
                    {t('unitRow.by', {
                      name: p.recordedBy?.username ?? t('unitRow.deletedUser'),
                    })}
                  </Typography>
                </TableCell>
              </TableRow>
            ))}
            {history.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} align="center">
                  {loading ? t('common.loading') : t('unitHistory.empty')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>
    </>
  );
}

/** Shows a unit's position history, newest measurement first. */
export default function UnitHistoryDialog({
  open,
  unit,
  onClose,
}: {
  open: boolean;
  unit: Unit | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  return (
    <Modal open={open} title={t('unitHistory.title', { name: unit?.name ?? '' })} onClose={onClose}>
      {unit && <HistoryTable unit={unit} />}
    </Modal>
  );
}
