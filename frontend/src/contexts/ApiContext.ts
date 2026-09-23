import { createContext } from 'react';
import type { ApiClient } from '../api/ApiClient';

export const ApiContext = createContext<ApiClient | null>(null);
