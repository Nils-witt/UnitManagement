import { Box, CircularProgress } from '@mui/material';
import './RouteFallback.scss';

/** Suspense fallback shown while a lazily loaded route chunk downloads. */
export default function RouteFallback() {
  return (
    <Box className="route-fallback">
      <CircularProgress />
    </Box>
  );
}
