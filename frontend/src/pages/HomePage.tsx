import { Paper, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../hooks/useAuth';
import './HomePage.scss';

export default function HomePage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  return (
    <Paper className="home-page">
      <Typography variant="h5" component="h1" gutterBottom>
        {t('home.welcome', { username: user?.username })}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t('home.loggedIn')}
      </Typography>
    </Paper>
  );
}
