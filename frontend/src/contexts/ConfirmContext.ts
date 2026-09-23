import { createContext } from 'react';

export interface ConfirmOptions {
  title?: string;
  message: string;
  /** Label of the confirming button. */
  confirmLabel?: string;
  /** Styles the confirming button as dangerous (deletions, revocations). */
  destructive?: boolean;
}

/** Asks the user to confirm; resolves true only if they did, false on cancel,
 * Escape or a click outside the dialog. */
export type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

export const ConfirmContext = createContext<ConfirmFn | null>(null);
