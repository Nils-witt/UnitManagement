import { useEffect } from 'react';
import { useInstanceName } from '../hooks/useInstanceName';

/** Keeps the browser tab title in sync with the instance name. */
export default function DocumentTitle() {
  const name = useInstanceName();
  useEffect(() => {
    document.title = name;
  }, [name]);
  return null;
}
