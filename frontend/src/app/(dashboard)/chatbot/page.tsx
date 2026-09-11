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

  /** Espejo del backend: models.MaxMonitorsPerConversation. */
  const MAX_MONITORS = 6;

  const activeConversation = conversations.find((c) => c.id === conversationId);

  /**
   * Un id que no resuelve puede ser dos cosas distintas y no hay que
   * confundirlas: mientras la lista de monitores todavía no llegó, ninguno
   * resuelve; una vez que llegó, un id que falta es un monitor BORRADO que
   * la conversación sigue referenciando — estado soportado en el backend
   * (chat_service.go lo excluye y sigue respondiendo con el resto), así
   * que nunca va a resolver. Mostrar el ObjectID crudo es ilegible, y
   * decir "Monitor eliminado" antes de tener la lista sería mentira.
   */
  function monitorNames(ids: string[]): string[] {
    const desconocido = monitorsLoaded ? "Monitor eliminado" : "Cargando…";
    return ids.map(
      (id) => monitors.find((m) => m.id === id)?.name ?? desconocido,
    );
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

            <div className="space-y-1">
              {conversations.map((c) =>
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
                        {monitorNames(c.monitorIds).join(" · ")}
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
              <div className="flex flex-wrap gap-1.5 border-b px-4 py-2">
                {monitorNames(activeConversation.monitorIds).map((name) => (
                  <span
                    key={name}
                    className="rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
                  >
                    {name}
                  </span>
                ))}
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
