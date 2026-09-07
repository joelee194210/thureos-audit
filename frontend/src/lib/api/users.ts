import { api } from "./client";
import type { User, Role } from "@/lib/types";

export const usersApi = {
  create: (data: {
    email: string;
    name: string;
    password: string;
    role: Role;
  }) => api.post<User>("/users", data),

  list: () => api.get<User[]>("/users"),

  get: (id: string) => api.get<User>(`/users/${id}`),

  updateRole: (id: string, role: Role) =>
    api.patch<{ message: string }>(`/users/${id}/role`, { role }),

  toggleActive: (id: string) =>
    api.patch<{ message: string; active: boolean }>(
      `/users/${id}/toggle-active`,
      {},
    ),

  delete: (id: string) => api.delete<{ message: string }>(`/users/${id}`),
};
