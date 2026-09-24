import { type FormEvent, useState } from 'react';
import { Button, FormControlLabel, Stack, Switch, TextField } from '@mui/material';
import { useTranslation } from 'react-i18next';
import type { User } from '../../api/types';
import ErrorBanner from '../../components/ErrorBanner';
import Modal from '../../components/Modal';
import { errorMessage } from '../../lib/errors';
import './UserDialog.scss';

/** Matches the server's minimum (auth.MinPasswordLength). */
const MIN_PASSWORD_LENGTH = 8;

export interface UserFormValues {
  username: string;
  /** Empty when editing means "keep the current password". */
  password: string;
  isAdmin: boolean;
}

/** Mounted by the modal only while it is open, so it starts fresh each time. */
function UserForm({
  user,
  self,
  onClose,
  onSubmit,
}: {
  user: User | null;
  self: boolean;
  onClose: () => void;
  onSubmit: (values: UserFormValues) => Promise<void>;
}) {
  const { t } = useTranslation();
  const editing = user != null;
  const [username, setUsername] = useState(user?.username ?? '');
  const [password, setPassword] = useState('');
  const [isAdmin, setIsAdmin] = useState(user?.isAdmin ?? false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const adminManaged = user?.adminManaged ?? false;
  const passwordTooShort = password !== '' && password.length < MIN_PASSWORD_LENGTH;
  const passwordHint = editing
    ? user.hasPassword
      ? t('userDialog.leaveBlankHint')
      : t('userDialog.ssoNoPasswordHint')
    : t('userDialog.minLengthHint', { count: MIN_PASSWORD_LENGTH });

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await onSubmit({ username: username.trim(), password, isAdmin });
      onClose();
    } catch (err) {
      setError(errorMessage(err));
      setSubmitting(false);
    }
  };

  return (
    <Stack component="form" spacing={3} className="user-dialog__form" onSubmit={handleSubmit}>
      <ErrorBanner message={error} />
      <Stack direction="row" spacing={2} useFlexGap className="user-dialog__credentials">
        <TextField
          label={t('login.usernameLabel')}
          size="small"
          required
          disabled={editing}
          autoFocus={!editing}
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
        <TextField
          label={
            editing && user.hasPassword
              ? t('userDialog.newPasswordLabel')
              : t('login.passwordLabel')
          }
          type="password"
          size="small"
          required={!editing}
          autoFocus={editing}
          autoComplete="new-password"
          error={passwordTooShort}
          helperText={passwordHint}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </Stack>
      <FormControlLabel
        control={
          <Switch
            checked={isAdmin}
            disabled={self || adminManaged}
            onChange={(e) => setIsAdmin(e.target.checked)}
          />
        }
        label={
          adminManaged
            ? t('userDialog.isAdminManaged')
            : self
              ? t('userDialog.isAdminSelf')
              : t('userDialog.isAdmin')
        }
      />
      <Stack direction="row" spacing={1} className="user-dialog__actions">
        <Button onClick={onClose} disabled={submitting}>
          {t('common.cancel')}
        </Button>
        <Button
          type="submit"
          variant="contained"
          disabled={submitting || passwordTooShort || (!editing && (!username.trim() || !password))}
        >
          {editing ? t('common.save') : t('common.create')}
        </Button>
      </Stack>
    </Stack>
  );
}

/** Creates a user (`user` is null) or edits an existing one's role and password. */
export default function UserDialog({
  open,
  user,
  self,
  onClose,
  onSubmit,
}: {
  open: boolean;
  user: User | null;
  self: boolean;
  onClose: () => void;
  onSubmit: (values: UserFormValues) => Promise<void>;
}) {
  const { t } = useTranslation();
  return (
    <Modal
      open={open}
      title={user ? t('userDialog.editTitle', { name: user.username }) : t('userDialog.newTitle')}
      onClose={onClose}
    >
      <UserForm user={user} self={self} onClose={onClose} onSubmit={onSubmit} />
    </Modal>
  );
}
