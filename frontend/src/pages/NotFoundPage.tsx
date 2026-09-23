import { useTranslation } from 'react-i18next';
import PageNotice from '../components/PageNotice';

export default function NotFoundPage() {
  const { t } = useTranslation();
  return <PageNotice message={t('notFound.message')} />;
}
