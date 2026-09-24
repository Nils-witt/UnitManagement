import { useTranslation } from 'react-i18next';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { InstanceInfo } from '../api/types';

const NO_INSTANCE: InstanceInfo = {};

/** The configured instance name (GET /api/instance), falling back to the
 * product name when none is set or it hasn't loaded yet. */
export function useInstanceName(): string {
  const { t } = useTranslation();
  const api = useApi();
  const { data } = useApiQuery(
    { queryKey: queryKeys.instance, queryFn: () => api.getInstance() },
    NO_INSTANCE,
  );
  return data.name || t('app.name');
}
