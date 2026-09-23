import { useCallback, useRef, useState, type ReactNode } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import { ConfirmContext, type ConfirmFn, type ConfirmOptions } from './ConfirmContext.ts';

/** Renders one shared confirmation dialog for `useConfirm()` callers,
 * replacing the browser's blocking `window.confirm`. */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const { t } = useTranslation();
  // `options` outlives `open` so the text doesn't vanish during the closing
  // transition.
  const [state, setState] = useState<{ open: boolean; options: ConfirmOptions }>({
    open: false,
    options: { message: '' },
  });
  const resolveRef = useRef<((confirmed: boolean) => void) | null>(null);

  const settle = useCallback((confirmed: boolean) => {
    resolveRef.current?.(confirmed);
    resolveRef.current = null;
    setState((s) => ({ ...s, open: false }));
  }, []);

  const confirm = useCallback<ConfirmFn>(
    (options) =>
      new Promise<boolean>((resolve) => {
        // A newer question replaces one that is still unanswered.
        resolveRef.current?.(false);
        resolveRef.current = resolve;
        setState({ open: true, options });
      }),
    [],
  );

  const { open, options } = state;
  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <Dialog
        open={open}
        onClose={() => settle(false)}
        maxWidth="xs"
        fullWidth
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-message"
      >
        <DialogTitle id="confirm-dialog-title">
          {options.title ?? t('confirmDialog.defaultTitle')}
        </DialogTitle>
        <DialogContent>
          <DialogContentText id="confirm-dialog-message">{options.message}</DialogContentText>
        </DialogContent>
        <DialogActions>
          {/* Focus starts on Cancel so Enter can't confirm a deletion by accident. */}
          <Button autoFocus onClick={() => settle(false)}>
            {t('confirmDialog.cancel')}
          </Button>
          <Button
            variant="contained"
            color={options.destructive ? 'error' : 'primary'}
            onClick={() => settle(true)}
          >
            {options.confirmLabel ?? t('confirmDialog.defaultConfirmLabel')}
          </Button>
        </DialogActions>
      </Dialog>
    </ConfirmContext.Provider>
  );
}
