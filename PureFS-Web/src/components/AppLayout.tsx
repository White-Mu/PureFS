import { useEffect } from 'react';
import { useAuthStore } from '../store';

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}

export function useInit() {
  const loadUser = useAuthStore((s) => s.loadUser);
  const token = useAuthStore((s) => s.token);

  useEffect(() => {
    if (token) loadUser();
  }, []);
}
