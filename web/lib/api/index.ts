import { RealApiClient } from './real';
import type { ApiClient } from './client';

export const api: ApiClient = new RealApiClient();
