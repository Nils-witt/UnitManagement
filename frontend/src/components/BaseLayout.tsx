import { Link, NavLink, Outlet } from 'react-router-dom';
import {
  AppBar,
  Box,
  Button,
  Container,
  IconButton,
  Menu,
  MenuItem,
  Toolbar,
  Tooltip,
  Typography,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import Footer from './Footer.tsx';
import LanguageSwitcher from './LanguageSwitcher.tsx';
import { useAuth } from '../hooks/useAuth.ts';
import { useInstanceName } from '../hooks/useInstanceName.ts';
import './BaseLayout.scss';
import MenuIcon from '@mui/icons-material/Menu';
import { useState, type MouseEvent } from 'react';
import { ROUTES } from '../routes.ts';

export default function BaseLayout() {
  const { t } = useTranslation();
  const instanceName = useInstanceName();
  const { user, logout } = useAuth();
  const [anchorElNav, setAnchorElNav] = useState<null | HTMLElement>(null);
  const [anchorElUser, setAnchorElUser] = useState<null | HTMLElement>(null);

  const tabs: { to: string; label: string }[] = [
    { to: ROUTES.home, label: t('nav.home') },
    { to: ROUTES.units, label: t('nav.units') },
    { to: ROUTES.map, label: t('nav.map') },
    ...(user?.isAdmin ? [{ to: ROUTES.users, label: t('nav.users') }] : []),
  ];

  const handleOpenNavMenu = (event: MouseEvent<HTMLElement>) => {
    setAnchorElNav(event.currentTarget);
  };
  const handleOpenUserMenu = (event: MouseEvent<HTMLElement>) => {
    setAnchorElUser(event.currentTarget);
  };

  const handleCloseNavMenu = () => {
    setAnchorElNav(null);
  };

  const handleCloseUserMenu = () => {
    setAnchorElUser(null);
  };

  const handleLogout = () => {
    handleCloseUserMenu();
    void logout();
  };

  return (
    <Box className="base-layout">
      <AppBar position="static" className="base-layout__app-bar">
        <Container maxWidth="xl">
          <Toolbar disableGutters>
            <Box className="base-layout__nav-toggle">
              <IconButton
                size="large"
                aria-label={t('nav.openNavMenu')}
                aria-controls="nav-menu"
                aria-haspopup="true"
                onClick={handleOpenNavMenu}
                color="inherit"
              >
                <MenuIcon />
              </IconButton>
              <Menu
                id="nav-menu"
                anchorEl={anchorElNav}
                anchorOrigin={{
                  vertical: 'bottom',
                  horizontal: 'left',
                }}
                transformOrigin={{
                  vertical: 'top',
                  horizontal: 'left',
                }}
                open={Boolean(anchorElNav)}
                onClose={handleCloseNavMenu}
              >
                {tabs.map((tab) => (
                  <MenuItem
                    key={tab.to}
                    component={NavLink}
                    to={tab.to}
                    end
                    onClick={handleCloseNavMenu}
                  >
                    {tab.label}
                  </MenuItem>
                ))}
              </Menu>
            </Box>

            <Typography
              variant="h6"
              noWrap
              component={Link}
              to={ROUTES.home}
              className="base-layout__brand"
            >
              {instanceName}
            </Typography>

            <Box className="base-layout__nav">
              {tabs.map((tab) => (
                <Button
                  key={tab.to}
                  component={NavLink}
                  to={tab.to}
                  end
                  className="base-layout__nav-link"
                >
                  {tab.label}
                </Button>
              ))}
            </Box>
            <Box className="base-layout__actions">
              <LanguageSwitcher />
              <Tooltip title={t('nav.account')}>
                <Button
                  onClick={handleOpenUserMenu}
                  aria-controls="user-menu"
                  aria-haspopup="true"
                  color="inherit"
                  className="base-layout__user-button"
                >
                  {user?.username}
                </Button>
              </Tooltip>
              <Menu
                id="user-menu"
                anchorEl={anchorElUser}
                anchorOrigin={{
                  vertical: 'bottom',
                  horizontal: 'right',
                }}
                transformOrigin={{
                  vertical: 'top',
                  horizontal: 'right',
                }}
                open={Boolean(anchorElUser)}
                onClose={handleCloseUserMenu}
              >
                <MenuItem onClick={handleLogout}>{t('nav.logout')}</MenuItem>
              </Menu>
            </Box>
          </Toolbar>
        </Container>
      </AppBar>

      <Container maxWidth={false} className="base-layout__content">
        <Outlet />
      </Container>
      <Footer />
    </Box>
  );
}
