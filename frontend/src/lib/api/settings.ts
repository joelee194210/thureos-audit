import { api } from "./client";

export interface AIConfigResponse {
  provider: string;
  model: string;
  apiKeyMasked: string;
  apiKeySet: boolean;
  baseUrl: string;
  availableModels: Record<string, string[]>;
  /**
   * Si `availableModels` del proveedor activo se leyó del proveedor mismo
   * o es la lista compilada de respaldo. La pantalla lo dice en voz alta:
   * un respaldo presentado como catálogo fue justo lo que dejó al chat
   * pidiendo un modelo retirado sin que nadie lo notara.
   */
  modelsAreLive?: boolean;
}

export interface NotificationSettings {
  emailProvider: "smtp" | "resend";
  smtp: {
    host: string;
    port: number;
    username: string;
    from: string;
    passwordSet: boolean;
  };
  resend: { from: string; apiKeySet: boolean };
  toEmails: string[];
  webhooks: {
    index: number;
    url: string;
    enabled: boolean;
    secretSet: boolean;
  }[];
}

export interface NotificationSettingsUpdate {
  emailProvider: "smtp" | "resend";
  smtp: {
    host: string;
    port: number;
    username: string;
    from: string;
    password?: string;
  };
  resend: { from: string; apiKey?: string };
  toEmails: string[];
  webhooks: { url: string; secret?: string; enabled: boolean }[];
}

export const settingsApi = {
  getAIConfig: () => api.get<AIConfigResponse>("/settings/ai"),
  updateAIConfig: (data: {
    provider: string;
    model: string;
    apiKey?: string;
    baseUrl?: string;
  }) => api.put<{ message: string }>("/settings/ai", data),

  getNotifications: () =>
    api.get<NotificationSettings>("/settings/notifications"),
  updateNotifications: (data: NotificationSettingsUpdate) =>
    api.put<{ message: string }>("/settings/notifications", data),
  testNotifications: () =>
    api.post<{ message: string }>("/settings/notifications/test", {}),
};
