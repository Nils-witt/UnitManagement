import type { ReactNode } from 'react';
import { Dialog, DialogContent, DialogTitle, IconButton, Stack } from '@mui/material';
import CloseIcon from '@mui/icons-material/Close';
import { useTranslation } from 'react-i18next';
import './Modal.scss';

interface ModalProps {
  open: boolean;
  title: string;
  onClose: () => void;
  headerExtra?: ReactNode;
  /** "auto" (default) sizes the modal to its content, up to 88vh — used by
   * every modal except the location picker, which needs a fixed, tall canvas
   * for MapLibre. */
  variant?: 'auto' | 'full';
  noPadding?: boolean;
  children: ReactNode;
}

export default function Modal({
  open,
  title,
  onClose,
  headerExtra,
  variant = 'auto',
  noPadding,
  children,
}: ModalProps) {
  const { t } = useTranslation();
  return (
    <Dialog open={open} onClose={onClose} maxWidth="lg" fullWidth fullScreen={variant === 'full'}>
      <DialogTitle className="modal__title">
        {title}
        <Stack direction="row" spacing={1} className="modal__header-extra">
          {headerExtra}
          <IconButton aria-label={t('common.close')} onClick={onClose} size="small">
            <CloseIcon fontSize="small" />
          </IconButton>
        </Stack>
      </DialogTitle>
      <DialogContent dividers className={noPadding ? 'modal__content--flush' : undefined}>
        {children}
      </DialogContent>
    </Dialog>
  );
}
