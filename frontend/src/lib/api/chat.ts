import { api } from "./client";

export interface ChartSpec {
  chartType?: "bar" | "line" | "pie";
  /**
   * Ausente cuando el dato viene de `sources`: el backend lo re-ejecuta
   * contra la data actual (ver `artifactsApi.run`) en vez de guardarlo acá.
   * Todo consumidor debe tratar `data` como opcional.
   */
  data?: Record<string, unknown>[];
  xKey?: string;
  yKeys?: string[];
  /** Proyección: qué columnas mostrar y en qué orden. Vacío = todas. */
  columns?: string[];
  /** Nombre de presentación por columna. No toca los datos. */
  labels?: Record<string, string>;
}

export type ChatArtifactType = "chart" | "table" | "custom";

/** Una fuente que alimenta un artefacto re-ejecutable. */
export interface ArtifactSource {
  monitor: string;
  label?: string;
}

export interface ChatArtifact {
  id?: string;
  type: ChatArtifactType;
  title: string;
  chartSpec?: ChartSpec;
  code?: string;
  /** Consultas que lo alimentan; su presencia es lo que lo hace re-ejecutable. */
  sources?: ArtifactSource[];
  monitorIds?: string[];
  saved?: boolean;
  savedName?: string;
  /** Última corrida conocida, para mostrar algo antes de volver a correrlo. */
  cachedData?: Record<string, unknown>[];
  ranAt?: string;
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
    api.getList<ChatConversation>(
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
    api.getList<ChatMessage>(`/chat/conversations/${conversationId}/messages`),

  ask: (conversationId: string, content: string) =>
    api.post<ChatMessage>(`/chat/conversations/${conversationId}/messages`, {
      content,
    }),

  deleteConversation: (conversationId: string) =>
    api.delete<{ message: string }>(`/chat/conversations/${conversationId}`),
};
