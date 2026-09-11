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
  monitorIds: string[];
  userId: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export const chatApi = {
  createConversation: (monitorIds: string[]) =>
    api.post<ChatConversation>("/chat/conversations", { monitorIds }),

  /** monitorId es un filtro opcional: sin él trae todas las del usuario. */
  listConversations: (monitorId?: string) =>
    api.get<ChatConversation[]>(
      monitorId
        ? `/chat/conversations?monitorId=${monitorId}`
        : "/chat/conversations",
    ),

  addMonitor: (conversationId: string, monitorId: string) =>
    api.post<ChatConversation>(
      `/chat/conversations/${conversationId}/monitors`,
      { monitorId },
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
