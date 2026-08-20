import { api } from "./client";

export interface AIConfigResponse {
  provider: string;
  model: string;
  apiKeyMasked: string;
  apiKeySet: boolean;
  baseUrl: string;
  availableModels: Record<string, string[]>;
}

export const settingsApi = {
  getAIConfig: () => api.get<AIConfigResponse>("/settings/ai"),
  updateAIConfig: (data: {
    provider: string;
    model: string;
    apiKey?: string;
    baseUrl?: string;
  }) => api.put<{ message: string }>("/settings/ai", data),
};
