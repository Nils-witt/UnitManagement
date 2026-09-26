import { lazy, Suspense, useMemo, useState } from 'react';
import { Route, Routes } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { CssBaseline, StyledEngineProvider, ThemeProvider, useMediaQuery } from '@mui/material';
import { AuthProvider } from './contexts/AuthProvider';
import ProtectedRoute from './auth/ProtectedRoute';
import AdminOnlyRoute from './auth/AdminOnlyRoute';
import RouteFallback from './components/RouteFallback';
import DocumentTitle from './components/DocumentTitle';
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
const GroupsPage = lazy(() => import('./pages/GroupsPage.tsx'));
const AuditLogPage = lazy(() => import('./pages/AuditLogPage.tsx'));
const UnitsPage = lazy(() => import('./pages/UnitsPage.tsx'));
const MapPage = lazy(() => import('./pages/MapPage.tsx'));

export default function App() {
  const prefersDarkMode = useMediaQuery('(prefers-color-scheme: dark)', { noSsr: true });
  const [queryClient] = useState(createQueryClient);
  const theme = useMemo(
    () => createAppTheme(prefersDarkMode ? 'dark' : 'light'),
    [prefersDarkMode],
  );

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
                <DocumentTitle />
                <Suspense fallback={<RouteFallback />}>
                  <Routes>
                    <Route path={ROUTE_SEGMENTS.login} element={<LoginPage />} />
                    <Route element={<ProtectedRoute />}>
                      <Route element={<BaseLayout />}>
                        <Route index element={<HomePage />} />
                        <Route path={ROUTE_SEGMENTS.units} element={<UnitsPage />} />
                        <Route path={ROUTE_SEGMENTS.map} element={<MapPage />} />
                        <Route element={<AdminOnlyRoute />}>
                          <Route path={ROUTE_SEGMENTS.users} element={<UsersPage />} />
                          <Route path={ROUTE_SEGMENTS.groups} element={<GroupsPage />} />
                          <Route path={ROUTE_SEGMENTS.auditLog} element={<AuditLogPage />} />
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
