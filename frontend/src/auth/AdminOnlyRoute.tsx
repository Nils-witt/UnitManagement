import { Outlet } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import PageNotice from '../components/PageNotice';
import { useAuth } from '../hooks/useAuth';

/** Guards the admin-only routes against being opened directly by URL: their
 * tab is already hidden from non-admins, but the route itself must also
 * refuse them. The server enforces the same rule on the API. */
export default function AdminOnlyRoute() {
  const { t } = useTranslation();
  const { user } = useAuth();
  if (user?.isAdmin) return <Outlet />;
  return <PageNotice message={t('adminOnly.message')} />;
}
