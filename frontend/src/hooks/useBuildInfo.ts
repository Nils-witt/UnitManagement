import { useApi } from './useApi';
import { queryKeys } from '../api/queryKeys';
import { useApiQuery } from './useApiQuery';
import type { VersionInfo } from '../api/types';

const NO_VERSION: VersionInfo = { commit: '' };

/** GET /api/version, formatted for the footer: "v1.2.3 · abc1234", just the
 * commit for untagged builds, or '' until it has loaded. */
export function useBuildInfo(): string {
  const api = useApi();
  const { data } = useApiQuery(
    { queryKey: queryKeys.version, queryFn: () => api.getVersion() },
    NO_VERSION,
  );
  return [data.version, data.commit].filter(Boolean).join(' · ');
}
