import { useCallback, useMemo } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { Settings } from '../api/types';

export interface SettingsData {
  settings: Settings;
  loading: boolean;
  error: string | null;
  /** PUT /api/settings (administrators only); the cache takes the stored result. */
  saveSettings: (settings: Settings) => Promise<void>;
}

const DEFAULT_SETTINGS: Settings = { mapStyleUrl: '' };

/** The instance-wide settings (GET /api/settings). */
export function useSettings(): SettingsData {
  const api = useApi();
  const queryClient = useQueryClient();
  const {
    data: settings,
    loading,
    error,
  } = useApiQuery(
    { queryKey: queryKeys.settings, queryFn: () => api.getSettings() },
    DEFAULT_SETTINGS,
  );
  const saveSettings = useCallback(
    async (next: Settings) => {
      const stored = await api.updateSettings(next);
      queryClient.setQueryData(queryKeys.settings, stored);
    },
    [api, queryClient],
  );
  return useMemo(
    () => ({ settings, loading, error, saveSettings }),
    [settings, loading, error, saveSettings],
  );
}
