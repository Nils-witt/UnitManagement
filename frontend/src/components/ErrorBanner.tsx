import { Alert } from '@mui/material';
import './ErrorBanner.scss';

export default function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <Alert severity="error" className="error-banner">
      {message}
    </Alert>
  );
}
