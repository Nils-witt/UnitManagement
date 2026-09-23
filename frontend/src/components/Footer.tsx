import { Box, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import './Footer.scss';

export default function Footer() {
  const { t } = useTranslation();
  return (
    <Box component="footer" className="footer">
      <Typography variant="caption" color="text.secondary">
        &copy; 2026 Nils Witt &middot; {t('app.name')}
      </Typography>
    </Box>
  );
}
