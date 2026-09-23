import { useMemo, useState } from 'react';
import { Box, Button, TextField } from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import { useTranslation } from 'react-i18next';
import type { Unit, UnitInput } from '../api/types';
import ErrorBanner from '../components/ErrorBanner';
import { useApi } from '../hooks/useApi';
import { useConfirm } from '../hooks/useConfirm';
import { useUnits } from '../hooks/useUnits';
import { errorMessage } from '../lib/errors';
import type { SortDirection } from '../lib/sort';
import UnitDialog from './units-page/UnitDialog';
import UnitsListCard from './units-page/UnitsListCard';
import { filterUnits, sortUnits, type UnitSortKey } from './units-page/unitSort';
import './units-page/UnitsToolbar.scss';

export default function UnitsPage() {
  const { t } = useTranslation();
  const api = useApi();
  const confirm = useConfirm();
  const { units, loading, error: loadError, reloadUnits } = useUnits();

  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState<UnitSortKey>('name');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  // The edited unit stays set while the dialog closes, so its form doesn't
  // flip to "new unit" during the exit animation.
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingUnit, setEditingUnit] = useState<Unit | null>(null);
  const [error, setError] = useState<string | null>(null);

  const visibleUnits = useMemo(
    () => sortUnits(filterUnits(units, search), sortKey, sortDirection),
    [units, search, sortKey, sortDirection],
  );

  const onSort = (key: UnitSortKey) => {
    if (key === sortKey) {
      setSortDirection((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      // Newest first is the useful default for dates.
      setSortDirection(key === 'updatedAt' ? 'desc' : 'asc');
    }
  };

  const openDialog = (unit: Unit | null) => {
    setEditingUnit(unit);
    setDialogOpen(true);
  };

  const onSubmit = async (input: UnitInput) => {
    if (editingUnit) {
      await api.updateUnit(editingUnit.id, input);
    } else {
      await api.createUnit(input);
    }
    await reloadUnits();
  };

  const onDelete = async (u: Unit) => {
    const ok = await confirm({
      title: t('units.deleteUnitTitle'),
      message: t('units.deleteUnitMessage', { name: u.name }),
      confirmLabel: t('common.delete'),
      destructive: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await api.deleteUnit(u.id);
      await reloadUnits();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  return (
    <Box>
      <div className="units-toolbar">
        <TextField
          label={t('units.searchLabel')}
          size="small"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => openDialog(null)}>
          {t('units.newUnit')}
        </Button>
      </div>
      <ErrorBanner message={error ?? loadError} />
      <UnitsListCard
        units={visibleUnits}
        loading={loading}
        sortKey={sortKey}
        sortDirection={sortDirection}
        onSort={onSort}
        onEdit={openDialog}
        onDelete={(u) => void onDelete(u)}
      />
      <UnitDialog
        open={dialogOpen}
        unit={editingUnit}
        onClose={() => setDialogOpen(false)}
        onSubmit={onSubmit}
      />
    </Box>
  );
}
