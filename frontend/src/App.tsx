import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { Route, Routes } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { CssBaseline, StyledEngineProvider, ThemeProvider, useMediaQuery } from '@mui/material';
import { useTranslation } from 'react-i18next';
import { AuthProvider } from './contexts/AuthProvider';
import ProtectedRoute from './auth/ProtectedRoute';
import AdminOnlyRoute from './auth/AdminOnlyRoute';
import RouteFallback from './components/RouteFallback';
import LoginPage from './pages/LoginPage';
import NotFoundPage from './pages/NotFoundPage';
import { createAppTheme } from './theme';
import BaseLayout from './components/BaseLayout.tsx';
import { ApiProvider } from './contexts/ApiProvider.tsx';
import { ConfirmProvider } from './contexts/ConfirmProvider.tsx';
import { createQueryClient } from './api/queryClient.ts';
import { ROUTE_SEGMENTS } from './routes.ts';

// Route-level components behind the login are code-split, so the login page
// doesn't download them.
const HomePage = lazy(() => import('./pages/HomePage.tsx'));
const UsersPage = lazy(() => import('./pages/UsersPage.tsx'));

export default function App() {
  const { t } = useTranslation();
  const prefersDarkMode = useMediaQuery('(prefers-color-scheme: dark)', { noSsr: true });
  const [queryClient] = useState(createQueryClient);
  const theme = useMemo(
    () => createAppTheme(prefersDarkMode ? 'dark' : 'light'),
    [prefersDarkMode],
  );

  useEffect(() => {
    document.title = t('app.name');
  }, [t]);

  // injectFirst puts MUI's styles before the component .scss files in the
  // cascade, so a plain single-class selector there wins over MUI's defaults.
  return (
    <StyledEngineProvider injectFirst>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <QueryClientProvider client={queryClient}>
          <ConfirmProvider>
            <AuthProvider>
              <ApiProvider>
                <Suspense fallback={<RouteFallback />}>
                  <Routes>
                    <Route path={ROUTE_SEGMENTS.login} element={<LoginPage />} />
                    <Route element={<ProtectedRoute />}>
                      <Route element={<BaseLayout />}>
                        <Route index element={<HomePage />} />
                        <Route element={<AdminOnlyRoute />}>
                          <Route path={ROUTE_SEGMENTS.users} element={<UsersPage />} />
                        </Route>
                        <Route path="*" element={<NotFoundPage />} />
                      </Route>
                    </Route>
                  </Routes>
                </Suspense>
              </ApiProvider>
            </AuthProvider>
          </ConfirmProvider>
        </QueryClientProvider>
      </ThemeProvider>
    </StyledEngineProvider>
  );
}
