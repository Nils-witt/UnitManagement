import { useState, type MouseEvent } from 'react';
import { IconButton, ListItemIcon, ListItemText, Menu, MenuItem, Tooltip } from '@mui/material';
import CheckIcon from '@mui/icons-material/Check';
import TranslateIcon from '@mui/icons-material/Translate';
import { useTranslation } from 'react-i18next';
import { SUPPORTED_LANGUAGES, type SupportedLanguage } from '../i18n/config';

const LANGUAGE_LABEL_KEYS: Record<SupportedLanguage, string> = {
  en: 'languageSwitcher.english',
  de: 'languageSwitcher.german',
};

/** A small menu button that switches the UI language; the choice is
 * remembered (see i18next-browser-languagedetector's localStorage cache). */
export default function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);

  const currentLanguage = (
    SUPPORTED_LANGUAGES.includes(i18n.language as SupportedLanguage) ? i18n.language : 'en'
  ) as SupportedLanguage;

  const handleOpen = (event: MouseEvent<HTMLElement>) => setAnchorEl(event.currentTarget);
  const handleClose = () => setAnchorEl(null);
  const handleSelect = (language: SupportedLanguage) => {
    void i18n.changeLanguage(language);
    handleClose();
  };

  return (
    <>
      <Tooltip title={t('languageSwitcher.label')}>
        <IconButton
          onClick={handleOpen}
          aria-controls="language-menu"
          aria-haspopup="true"
          aria-label={t('languageSwitcher.label')}
          color="inherit"
        >
          <TranslateIcon />
        </IconButton>
      </Tooltip>
      <Menu id="language-menu" anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={handleClose}>
        {SUPPORTED_LANGUAGES.map((language) => (
          <MenuItem
            key={language}
            onClick={() => handleSelect(language)}
            selected={language === currentLanguage}
          >
            {language === currentLanguage && (
              <ListItemIcon>
                <CheckIcon fontSize="small" />
              </ListItemIcon>
            )}
            <ListItemText inset={language !== currentLanguage}>
              {t(LANGUAGE_LABEL_KEYS[language])}
            </ListItemText>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
