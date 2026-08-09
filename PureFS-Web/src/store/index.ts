import { create } from 'zustand';
import { auth } from '../api';
import type { User } from '../api';

interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
  login: (username: string, password: string, totpCode?: string) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: JSON.parse(localStorage.getItem('user') || 'null'),
  token: localStorage.getItem('token'),
  loading: false,

  login: async (username, password, totpCode) => {
    const res = await auth.login({ username, password, totp_code: totpCode });
    if (res.totp_required) {
      throw new Error('TOTP_REQUIRED');
    }
    localStorage.setItem('token', res.token);
    localStorage.setItem('user', JSON.stringify(res.user));
    set({ token: res.token, user: res.user });
  },

  logout: () => {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    set({ token: null, user: null });
  },

  loadUser: async () => {
    try {
      const user = await auth.me();
      localStorage.setItem('user', JSON.stringify(user));
      set({ user });
    } catch {
      set({ user: null, token: null });
    }
  },
}));

interface UIState {
  sidebarOpen: boolean;
  viewMode: 'list' | 'grid' | 'timeline';
  darkMode: boolean;
  toggleSidebar: () => void;
  setViewMode: (mode: 'list' | 'grid' | 'timeline') => void;
  toggleDarkMode: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  sidebarOpen: true,
  viewMode: 'list',
  darkMode: localStorage.getItem('darkMode') === 'true',
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),
  setViewMode: (mode) => set({ viewMode: mode }),
  toggleDarkMode: () =>
    set((s) => {
      const next = !s.darkMode;
      localStorage.setItem('darkMode', String(next));
      document.documentElement.setAttribute('data-theme', next ? 'dark' : 'light');
      return { darkMode: next };
    }),
}));
