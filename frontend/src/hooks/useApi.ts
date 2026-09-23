import { useContext } from 'react';
import type { ApiClient } from '../api/ApiClient.ts';
import { ApiContext } from '../contexts/ApiContext.ts';

export function useApi(): ApiClient {
  const ctx = useContext(ApiContext);
  if (!ctx) throw new Error('useApi must be used within ApiProvider');
  return ctx;
}
