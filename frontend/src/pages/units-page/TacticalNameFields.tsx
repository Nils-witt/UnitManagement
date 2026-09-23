import { Stack, TextField } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { TacticalName } from '../../api/types';
import { MAX_TACTICAL_NAME_PART_LENGTH, tacticalNameParts } from '../../lib/tacticalName';
import './TacticalNameFields.scss';

export default function TacticalNameFields({
  value,
  onChange,
}: {
  value: TacticalName;
  onChange: (value: TacticalName) => void;
}) {
  const { t } = useTranslation();
  return (
    <Stack direction="row" spacing={2} useFlexGap className="tactical-name-fields">
      {tacticalNameParts.map((key) => (
        <TextField
          key={key}
          size="small"
          label={t(`tacticalName.${key}`)}
          value={value[key] ?? ''}
          onChange={(e) => onChange({ ...value, [key]: e.target.value })}
          slotProps={{ htmlInput: { maxLength: MAX_TACTICAL_NAME_PART_LENGTH } }}
          className={`tactical-name-fields__${key === 'function' || key === 'number' ? 'short' : 'long'}`}
        />
      ))}
    </Stack>
  );
}
