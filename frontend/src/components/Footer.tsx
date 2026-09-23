import { Box, Typography } from '@mui/material';
import { useTranslation } from 'react-i18next';
import { useBuildInfo } from '../hooks/useBuildInfo';
import './Footer.scss';

export default function Footer() {
  const { t } = useTranslation();
  const buildInfo = useBuildInfo();
  return (
    <Box component="footer" className="footer">
      <Typography variant="caption" color="text.secondary">
        &copy; 2026 Nils Witt &middot; {t('app.name')}
        {buildInfo && <> &middot; {buildInfo}</>}
      </Typography>
    </Box>
  );
}
