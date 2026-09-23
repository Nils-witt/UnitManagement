import { MenuItem, Stack, TextField } from '@mui/material';
import {
  einheiten,
  fachaufgaben,
  funktionen,
  grundzeichen,
  organisationen,
  symbole,
  verwaltungsstufen,
} from '@taktische-zeichen/core';
import { useTranslation } from 'react-i18next';
import type { UnitSymbol } from '../../api/types';
import UnitSymbolIcon from '../../components/UnitSymbolIcon';
import { acceptsComponent, cleanSymbol, type SymbolComponent } from '../../lib/unitSymbol';
import './UnitSymbolFields.scss';

interface Option {
  id: string;
  label: string;
  deprecated?: boolean | string;
}

const components: { key: SymbolComponent; options: Option[] }[] = [
  { key: 'organisation', options: organisationen },
  { key: 'fachaufgabe', options: fachaufgaben },
  { key: 'einheit', options: einheiten },
  { key: 'verwaltungsstufe', options: verwaltungsstufen },
  { key: 'funktion', options: funktionen },
  { key: 'symbol', options: symbole },
];

/** Library labels are German; deprecated entries are only kept while selected. */
function OptionSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string | undefined;
  options: Option[];
  onChange: (value: string | undefined) => void;
}) {
  const { t } = useTranslation();
  return (
    <TextField
      select
      size="small"
      label={label}
      value={value ?? ''}
      onChange={(e) => onChange(e.target.value || undefined)}
      className="unit-symbol-fields__select"
    >
      <MenuItem value="">
        <em>{t('unitSymbol.none')}</em>
      </MenuItem>
      {options
        .filter((o) => !o.deprecated || o.id === value)
        .map((o) => (
          <MenuItem key={o.id} value={o.id}>
            {o.label}
          </MenuItem>
        ))}
    </TextField>
  );
}

export default function UnitSymbolFields({
  value,
  onChange,
}: {
  value: UnitSymbol;
  onChange: (value: UnitSymbol) => void;
}) {
  const { t } = useTranslation();
  const set = (key: keyof UnitSymbol, v: string | undefined) => onChange({ ...value, [key]: v });
  return (
    <Stack direction="row" spacing={2} useFlexGap className="unit-symbol-fields">
      <UnitSymbolIcon symbol={cleanSymbol(value)} size="large" />
      <Stack direction="row" spacing={2} useFlexGap className="unit-symbol-fields__selects">
        <OptionSelect
          label={t('unitSymbol.grundzeichen')}
          value={value.grundzeichen}
          options={grundzeichen}
          onChange={(v) => set('grundzeichen', v)}
        />
        {components
          .filter(({ key }) => acceptsComponent(value.grundzeichen, key))
          .map(({ key, options }) => (
            <OptionSelect
              key={key}
              label={t(`unitSymbol.${key}`)}
              value={value[key]}
              options={options}
              onChange={(v) => set(key, v)}
            />
          ))}
      </Stack>
    </Stack>
  );
}
