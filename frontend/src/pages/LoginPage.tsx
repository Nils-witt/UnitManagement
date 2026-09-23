import { useEffect, useState, type SubmitEvent } from 'react';
import { Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import {
  Alert,
  Box,
  Button,
  Container,
  Divider,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import LoginIcon from '@mui/icons-material/Login';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/ApiClient';
import { useApi } from '../hooks/useApi';
import { useAuth } from '../hooks/useAuth';
import { useAuthMethods } from '../hooks/useAuthMethods';
import Footer from '../components/Footer';
import LanguageSwitcher from '../components/LanguageSwitcher';
import './LoginPage.scss';
import { ROUTES } from '../routes';

const SSO_ERROR_KEYS: Record<string, string> = {
  denied: 'login.ssoErrors.denied',
  expired: 'login.ssoErrors.expired',
  failed: 'login.ssoErrors.failed',
};

export default function LoginPage() {
  const { t } = useTranslation();
  const auth = useAuth();
  const routerLocation = useLocation();
  const from = (routerLocation.state as { from?: string } | null)?.from ?? null;

  const navigate = useNavigate();
  const api = useApi();
  const authMethods = useAuthMethods();
  const [searchParams, setSearchParams] = useSearchParams();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [submitting, setSubmitting] = useState(false);
  // A failed SSO sign-in comes back as /login?sso_error=<code>.
  const [error, setError] = useState<string | null>(() => {
    const code = searchParams.get('sso_error');
    if (!code) return null;
    return t(SSO_ERROR_KEYS[code] ?? SSO_ERROR_KEYS.failed);
  });

  useEffect(() => {
    // Drop the code from the URL so a reload doesn't show the error again.
    if (searchParams.has('sso_error')) setSearchParams({}, { replace: true });
  }, [searchParams, setSearchParams]);

  useEffect(() => {
    if (auth.sessionMessage) {
      setError(auth.sessionMessage);
      auth.clearSessionMessage();
    }
    // Only re-run when the session-expired message actually changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auth.sessionMessage]);

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await auth.login(username, password);
      await navigate(from ?? ROUTES.home, { replace: true });
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 401
          ? t('login.invalidCredentials')
          : t('login.loginFailed'),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const handleSSO = () => {
    window.location.href = api.oidcLoginUrl(from ?? ROUTES.home);
  };

  // Opening /login with a live session (bookmark, back button) goes home.
  if (auth.isAuthenticated && !submitting) {
    return <Navigate to={from ?? ROUTES.home} replace />;
  }

  return (
    <Box className="login-page">
      <Box className="login-page__language-switcher">
        <LanguageSwitcher />
      </Box>
      <Container maxWidth="xs" disableGutters>
        <Paper elevation={3}>
          <Typography variant="h5" component="h1" gutterBottom sx={{ textAlign: 'center' }}>
            {t('app.name')}
          </Typography>
          <Box component="form" onSubmit={handleSubmit} noValidate>
            <Stack spacing={2}>
              <TextField
                id="username"
                label={t('login.usernameLabel')}
                autoComplete="username"
                required
                fullWidth
                autoFocus
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
              <TextField
                id="password"
                label={t('login.passwordLabel')}
                type="password"
                autoComplete="current-password"
                required
                fullWidth
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
              <Button
                type="submit"
                variant="contained"
                fullWidth
                disabled={submitting || !username || !password}
                startIcon={<LoginIcon />}
              >
                {submitting ? t('login.signingIn') : t('login.signIn')}
              </Button>
            </Stack>
          </Box>
          {authMethods.oidc && (
            <>
              <Divider>{t('login.or')}</Divider>
              <Button type="button" variant="outlined" fullWidth onClick={handleSSO}>
                {t('login.signInWith', { name: authMethods.oidcName ?? 'SSO' })}
              </Button>
            </>
          )}
          {error && <Alert severity="error">{error}</Alert>}
        </Paper>
      </Container>
      <Footer />
    </Box>
  );
}
