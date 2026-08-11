import api from './client';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

export interface FileItem {
  id: number;
  name: string;
  path: string;
  file_type: 'file' | 'directory' | 'symlink';
  mime_type: string;
  size: number;
  sha256: string;
  is_pinned: boolean;
  is_favorite: boolean;
  is_e2ee?: boolean;
  is_encrypted?: boolean;
  dek_ciphertext?: string;
  kek_version?: number;
  created_at: string;
  updated_at: string;
}

export interface FileListResponse {
  items: FileItem[];
  total: number;
}

export interface User {
  id: number;
  username: string;
  email: string;
  role: string;
  storage_quota: number;
  storage_used: number;
  is_active: boolean;
  totp_enabled?: boolean;
}

export interface AuditLogEntry {
  id: number;
  user_id: number;
  action: string;
  detail: string;
  ip: string;
  created_at: string;
}

export interface LoginResponse {
  token: string;
  user: User;
  totp_required?: boolean;
}

export interface Share {
  id: number;
  file_id: number;
  file_name?: string;
  token: string;
  expires_at: string;
  max_accesses: number;
  access_count: number;
  can_download: boolean;
  is_active: boolean;
  created_at: string;
}

export interface E2EEStatus {
  enabled: boolean;
  salt?: string;
  wrapped_key?: string;
}

export const auth = {
  login: (data: { username: string; password: string; totp_code?: string }) =>
    api.post<LoginResponse>('/auth/login', data).then(r => r.data),

  register: (data: { username: string; email: string; password: string }) =>
    api.post<User>('/auth/register', data).then(r => r.data),

  me: () => api.get<User>('/users/me').then(r => r.data),

  refresh: () => api.post<LoginResponse>('/auth/refresh').then(r => r.data),

  setupTOTP: () => api.post<{ secret: string; uri: string }>('/auth/totp/setup').then(r => r.data),

  enableTOTP: (code: string) => api.post('/auth/totp/enable', { code }),

  disableTOTP: () => api.post('/auth/totp/disable'),

  e2eeStatus: () => api.get<E2EEStatus>('/users/e2ee/status').then(r => r.data),

  e2eeSetup: (data: { salt: string; wrapped_key: string }) =>
    api.post('/users/e2ee', data),

  e2eeDisable: () => api.delete('/users/e2ee'),
};

export const files = {
  list: (params?: { parent_id?: number; sort_by?: string; sort_order?: string; search?: string; offset?: number; limit?: number; file_type?: string; view?: string; is_favorite?: boolean; is_pinned?: boolean }) =>
    api.get<FileListResponse>('/files', { params }).then(r => r.data),

  get: (id: number) => api.get<FileItem>(`/files/${id}`).then(r => r.data),

  download: (id: number) => `${API_BASE}/api/files/${id}/download`,

  downloadBlob: (id: number) => {
    const token = localStorage.getItem('token');
    return api.get<Blob>(`/files/${id}/download?token=${token}`, { responseType: 'blob' }).then(r => r.data);
  },

  downloadBlobUrl: (id: number) => {
    const token = localStorage.getItem('token');
    return `${API_BASE}/api/files/${id}/download?token=${token}`;
  },

  createDir: (data: { parent_id?: number; name: string; file_type: string }) =>
    api.post<FileItem>('/files/dir', data).then(r => r.data),

  upload: (file: File, parentId?: number) => {
    const form = new FormData();
    form.append('file', file);
    if (parentId) form.append('parent_id', String(parentId));
    return api.post<FileItem>('/files/upload', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then(r => r.data);
  },

  uploadE2EE: (file: Blob, name: string, parentId: number | undefined, dekCiphertext: string) => {
    const form = new FormData();
    form.append('file', file, name);
    if (parentId) form.append('parent_id', String(parentId));
    form.append('is_e2ee', 'true');
    form.append('dek_ciphertext', dekCiphertext);
    form.append('kek_version', '0');
    return api.post<FileItem>('/files/upload', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then(r => r.data);
  },

  rename: (id: number, name: string) =>
    api.patch<FileItem>(`/files/${id}/rename`, { name }).then(r => r.data),

  move: (id: number, targetParentId: number) =>
    api.patch<FileItem>(`/files/${id}/move`, { target_parent_id: targetParentId }).then(r => r.data),

  delete: (id: number) => api.delete(`/files/${id}`),

  setPinned: (id: number, pinned: boolean) =>
    api.patch(`/files/${id}/pin`, { pinned }),

  setFavorite: (id: number, favorite: boolean) =>
    api.patch(`/files/${id}/favorite`, { favorite }),

  copy: (id: number, data?: { target_parent_id?: number; new_name?: string }) =>
    api.post<FileItem>(`/files/${id}/copy`, data || {}).then(r => r.data),

  batchDelete: (ids: number[]) =>
    api.post<{ deleted: number; failed: number[] }>('/files/batch/delete', { ids }).then(r => r.data),
};

export const shares = {
  create: (data: { file_id: number; password?: string; expires_in?: string; max_accesses?: number; can_download?: boolean }) =>
    api.post<Share>('/shares', data).then(r => r.data),

  list: () => api.get<Share[]>('/shares').then(r => r.data),

  get: (token: string, password?: string) =>
    api.get(`/shares/${token}`, { params: { password } }).then(r => r.data),

  deactivate: (id: number) => api.delete(`/shares/${id}`),
};

export const admin = {
  auditLogs: (params?: { limit?: number; offset?: number }) =>
    api.get<AuditLogEntry[]>('/admin/audit-logs', { params }).then(r => r.data),

  listUsers: () => api.get<User[]>('/users').then(r => r.data),

  permissions: {
    list: (userId: number) =>
      api.get<Permission[]>('/admin/permissions', { params: { user_id: userId } }).then(r => r.data),

    create: (data: { user_id: number; file_path: string; perm: string }) =>
      api.post<Permission>('/admin/permissions', data).then(r => r.data),

    delete: (id: number) => api.delete(`/admin/permissions/${id}`),
  },
};

export interface Permission {
  id: number;
  user_id: number;
  file_path: string;
  perm: string;
}

export interface RecycleBinItem {
  id: number;
  user_id: number;
  file_id: number;
  original_path: string;
  original_name: string;
  trash_path: string;
  file_type: string;
  file_size: number;
  is_dir: number;
  deleted_at: string;
  expire_at: string;
}
