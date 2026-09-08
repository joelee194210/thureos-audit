import { api } from "./client";

export interface ChartSpec {
  chartType?: "bar" | "line" | "pie";
  data: Record<string, unknown>[];
  xKey?: string;
  yKeys?: string[];
}

export type ChatArtifactType = "chart" | "table" | "custom";

export interface ChatArtifact {
  type: ChatArtifactType;
  title: string;
  chartSpec?: ChartSpec;
  code?: string;
}

export type ChatRole = "user" | "assistant";

export interface ChatMessage {
  id: string;
  conversationId: string;
  role: ChatRole;
  content: string;
  artifact?: ChatArtifact;
  createdAt: string;
}

export interface ChatConversation {
  id: string;
  monitorId: string;
  userId: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export const chatApi = {
  createConversation: (monitorId: string) =>
    api.post<ChatConversation>("/chat/conversations", { monitorId }),

  listConversations: (monitorId: string) =>
    api.get<ChatConversation[]>(
      `/chat/conversations?monitorId=${monitorId}`,
    ),

  listMessages: (conversationId: string) =>
    api.get<ChatMessage[]>(`/chat/conversations/${conversationId}/messages`),

  ask: (conversationId: string, content: string) =>
    api.post<ChatMessage>(`/chat/conversations/${conversationId}/messages`, {
      content,
    }),

  deleteConversation: (conversationId: string) =>
    api.delete<{ message: string }>(`/chat/conversations/${conversationId}`),
};
