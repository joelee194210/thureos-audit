import { create } from "zustand";
import { api } from "@/lib/api/client";
import type { User } from "@/lib/types";

interface AuthState {
  user: User | null;
  token: string | null;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
  loadUser: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  token: typeof window !== "undefined" ? localStorage.getItem("token") : null,
  isLoading: false,

  login: async (email, password) => {
    set({ isLoading: true });
    try {
      const res = await api.post<{ token: string; user: User }>("/auth/login", {
        email,
        password,
      });
      api.setToken(res.token);
      set({ user: res.user, token: res.token, isLoading: false });
    } catch (error) {
      set({ isLoading: false });
      throw error;
    }
  },

  register: async (name, email, password) => {
    set({ isLoading: true });
    try {
      const res = await api.post<{ token: string; user: User }>("/auth/register", {
        name,
        email,
        password,
      });
      api.setToken(res.token);
      set({ user: res.user, token: res.token, isLoading: false });
    } catch (error) {
      set({ isLoading: false });
      throw error;
    }
  },

  logout: () => {
    api.clearToken();
    set({ user: null, token: null });
  },

  loadUser: async () => {
    const token = api.getToken();
    if (!token) return;

    try {
      const user = await api.get<User>("/auth/me");
      set({ user, token });
    } catch {
      api.clearToken();
      set({ user: null, token: null });
    }
  },
}));
