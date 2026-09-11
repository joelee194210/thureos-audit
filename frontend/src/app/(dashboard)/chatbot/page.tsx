"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Bot, Send, Plus, Trash2 } from "lucide-react";
import { useToast } from "@/lib/use-toast";
import { monitorsApi } from "@/lib/api/monitors";
import type { Monitor } from "@/lib/types";
import {
  chatApi,
  type ChatConversation,
  type ChatMessage,
} from "@/lib/api/chat";
import { ArtifactCanvas } from "@/components/chat/artifact-canvas";
import { MonitorMultiSelect } from "@/components/chat/monitor-multi-select";
import { AddMonitorButton } from "@/components/chat/add-monitor-button";

export default function ChatbotPage() {
  const { toastError } = useToast();

  const [monitors, setMonitors] = useState<Monitor[]>([]);
  // Solo se marca en el camino feliz: si la carga falla, la lista vacía no
  // es evidencia de que los monitores no existan.
  const [monitorsLoaded, setMonitorsLoaded] = useState(false);

  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [activeArtifact, setActiveArtifact] = useState<
    ChatMessage["artifact"] | null
  >(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [creating, setCreating] = useState(false);
  const [draftMonitorIds, setDraftMonitorIds] = useState<string[]>([]);

  // Filtro del sidebar. Se aplica del lado del cliente sobre la lista
  // completa en vez de pedirla filtrada al backend: listConversations()
  // ya devuelve todas las del usuario sin paginación, así que no hay nada
  // que ahorrar, y —más importante— activeConversation se deriva de
  // `conversations`. Si el filtro achicara ese arreglo, la conversación
  // abierta desaparecería de él y la cabecera se quedaría sin chips
  // aunque el hilo siguiera abierto. El filtro del backend
  // (listConversations(monitorId)) queda disponible para cuando haya
  // paginación y el cliente deje de tenerlas todas.
  const [filterMonitorId, setFilterMonitorId] = useState("");

  /** Espejo del backend: models.MaxMonitorsPerConversation. */
  const MAX_MONITORS = 6;

  const activeConversation = conversations.find((c) => c.id === conversationId);

  const visibleConversations = filterMonitorId
    ? conversations.filter((c) => c.monitorIds.includes(filterMonitorId))
    : conversations;

  /**
   * Un id que no resuelve puede ser dos cosas distintas y no hay que
   * confundirlas: mientras la lista de monitores todavía no llegó, ninguno
   * resuelve; una vez que llegó, un id que falta es un monitor BORRADO que
   * la conversación sigue referenciando — estado soportado en el backend
   * (chat_service.go lo excluye y sigue respondiendo con el resto), así
   * que nunca va a resolver. Mostrar el ObjectID crudo es ilegible, y
   * decir "Monitor eliminado" antes de tener la lista sería mentira.
   *
   * Devuelve el id junto al nombre porque el nombre NO sirve como key de
   * React: dos monitores pueden llamarse igual, y mientras cargan todos
   * caen al mismo texto "Cargando…".
   */
  function monitorNames(ids: string[]): { id: string; name: string }[] {
    const desconocido = monitorsLoaded ? "Monitor eliminado" : "Cargando…";
    return ids.map((id) => ({
      id,
      name: monitors.find((m) => m.id === id)?.name ?? desconocido,
    }));
  }

  useEffect(() => {
    monitorsApi
      .list()
      .then((data) => {
        setMonitors(data);
        setMonitorsLoaded(true);
      })
      .catch(() => toastError("Error al cargar monitores"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    chatApi
      .listConversations()
      .then(setConversations)
      .catch(() => toastError("Error al cargar conversaciones"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!conversationId) return;
    chatApi
      .listMessages(conversationId)
      .then((msgs) => {
        setMessages(msgs);
        const lastWithArtifact = [...msgs].reverse().find((m) => m.artifact);
        setActiveArtifact(lastWithArtifact?.artifact ?? null);
      })
      .catch(() => toastError("Error al cargar la conversación"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conversationId]);

  async function confirmDeleteConversation(id: string) {
    try {
      await chatApi.deleteConversation(id);
      setConversations((prev) => prev.filter((c) => c.id !== id));
      if (id === conversationId) {
        setConversationId(null);
        setMessages([]);
        setActiveArtifact(null);
      }
    } catch {
      toastError("Error al borrar la conversación");
    } finally {
      setDeletingId(null);
    }
  }

  async function newConversation() {
    if (draftMonitorIds.length === 0) return;
    try {
      const conv = await chatApi.createConversation(draftMonitorIds);
      setConversations((prev) => [conv, ...prev]);
      setConversationId(conv.id);
      setMessages([]);
      setActiveArtifact(null);
      setCreating(false);
      setDraftMonitorIds([]);
    } catch {
      toastError("Error al crear la conversación");
    }
  }

  /**
   * Suma un monitor a la conversación abierta. El hilo no cambia: las
   * respuestas anteriores se dieron sin este monitor y no se reescriben.
   *
   * El backend usa compare-and-set, así que puede responder que la
   * conversación cambió mientras tanto — el caso real es tener dos
   * pestañas abiertas. Ahí no se reintenta solo: el estado que el usuario
   * vio ya no es el verdadero, así que se recarga la lista y se le dice.
   */
  async function addMonitor(monitorId: string) {
    if (!conversationId) return;
    try {
      const updated = await chatApi.addMonitor(conversationId, monitorId);
      setConversations((prev) =>
        prev.map((c) => (c.id === updated.id ? updated : c)),
      );
    } catch {
      toastError(
        "No se pudo agregar el monitor. La conversación pudo haber cambiado en otra pestaña; se recargó la lista.",
      );
      chatApi
        .listConversations()
        .then(setConversations)
        .catch(() => toastError("Error al recargar las conversaciones"));
    }
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim() || !conversationId) return;

    const question = input;
    setInput("");
    setMessages((prev) => [
      ...prev,
      {
        id: `local-${Date.now()}`,
        conversationId,
        role: "user",
        content: question,
        createdAt: new Date().toISOString(),
      },
    ]);
    setSending(true);
    try {
      const answer = await chatApi.ask(conversationId, question);
      setMessages((prev) => [...prev, answer]);
      if (answer.artifact) setActiveArtifact(answer.artifact);
    } catch {
      toastError("Error al consultar al chatbot");
    } finally {
      setSending(false);
    }
  }

  return (
    <>
      <Header title="Analista IA" />
      <div className="p-6 space-y-4">
        <div
          className={`grid gap-4 ${
            activeArtifact
              ? "grid-cols-[220px_1fr_1fr]"
              : "grid-cols-[220px_1fr]"
          }`}
        >
          <div className="space-y-2">
            <Button
              variant="outline"
              size="sm"
              className="w-full justify-start gap-2"
              onClick={() => setCreating((v) => !v)}
            >
              <Plus className="h-4 w-4" /> Nueva conversación
            </Button>

            {creating && (
              <div className="space-y-2 rounded-md border p-2">
                <MonitorMultiSelect
                  monitors={monitors}
                  selected={draftMonitorIds}
                  onChange={setDraftMonitorIds}
                  max={MAX_MONITORS}
                />
                <Button
                  size="sm"
                  className="w-full"
                  disabled={draftMonitorIds.length === 0}
                  onClick={newConversation}
                >
                  Empezar
                </Button>
              </div>
            )}

            {conversations.length > 0 && (
              <select
                value={filterMonitorId}
                onChange={(e) => setFilterMonitorId(e.target.value)}
                aria-label="Filtrar conversaciones por monitor"
                className="h-8 w-full rounded-md border bg-background px-2 text-xs"
              >
                <option value="">Todos los monitores</option>
                {monitors.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name}
                  </option>
                ))}
              </select>
            )}

            <div className="space-y-1">
              {filterMonitorId && visibleConversations.length === 0 && (
                <p className="px-2 py-1.5 text-xs text-muted-foreground">
                  Ninguna conversación incluye ese monitor.
                </p>
              )}
              {visibleConversations.map((c) =>
                deletingId === c.id ? (
                  <div
                    key={c.id}
                    className="flex items-center gap-1 rounded-md bg-destructive/10 px-2 py-1.5 text-xs"
                  >
                    <span className="flex-1 truncate text-muted-foreground">
                      ¿Borrar?
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 px-1.5 text-destructive hover:text-destructive"
                      onClick={() => confirmDeleteConversation(c.id)}
                    >
                      Sí
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-6 px-1.5"
                      onClick={() => setDeletingId(null)}
                    >
                      No
                    </Button>
                  </div>
                ) : (
                  <div key={c.id} className="group flex items-center">
                    <button
                      onClick={() => setConversationId(c.id)}
                      className={`flex-1 min-w-0 text-left rounded-md px-2 py-1.5 text-sm truncate ${
                        c.id === conversationId
                          ? "bg-accent text-accent-foreground"
                          : "hover:bg-accent/50"
                      }`}
                    >
                      {c.title}
                      <span className="mt-0.5 block truncate text-[11px] text-muted-foreground">
                        {monitorNames(c.monitorIds)
                          .map((m) => m.name)
                          .join(" · ")}
                      </span>
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeletingId(c.id);
                      }}
                      className="shrink-0 rounded-sm p-1 text-muted-foreground opacity-0 hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100"
                      aria-label={`Borrar conversación ${c.title}`}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ),
              )}
            </div>
          </div>

          <Card className="flex flex-col h-[70vh]">
            {activeConversation && (
              <div className="flex flex-wrap items-center gap-1.5 border-b px-4 py-2">
                {monitorNames(activeConversation.monitorIds).map((m) => (
                  <span
                    key={m.id}
                    className="rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
                  >
                    {m.name}
                  </span>
                ))}
                <AddMonitorButton
                  monitors={monitors}
                  selected={activeConversation.monitorIds}
                  onAdd={addMonitor}
                  max={MAX_MONITORS}
                  disabled={sending}
                />
              </div>
            )}
            <CardContent className="flex-1 overflow-y-auto p-4 space-y-3">
              {messages.length === 0 && (
                <p className="text-sm text-muted-foreground flex items-center gap-2">
                  <Bot className="h-4 w-4" />
                  {conversationId
                    ? "Preguntame algo sobre estos monitores."
                    : "Creá una conversación y elegí sobre qué monitores querés preguntar."}
                </p>
              )}
              {messages.map((m) => (
                <div
                  key={m.id}
                  className={`rounded-md px-3 py-2 text-sm max-w-[80%] whitespace-pre-wrap ${
                    m.role === "user"
                      ? "ml-auto bg-primary text-primary-foreground"
                      : "bg-muted"
                  }`}
                >
                  {m.content}
                </div>
              ))}
              {sending && (
                <p className="text-sm text-muted-foreground">Pensando…</p>
              )}
            </CardContent>
            <form
              onSubmit={send}
              className="flex gap-2 border-t p-3"
            >
              <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder="Preguntá algo sobre los datos de estos monitores"
                disabled={sending || !conversationId}
              />
              <Button type="submit" disabled={sending || !input.trim() || !conversationId}>
                <Send className="h-4 w-4" />
              </Button>
            </form>
          </Card>

          {activeArtifact && <ArtifactCanvas artifact={activeArtifact} />}
        </div>
      </div>
    </>
  );
}
