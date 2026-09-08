"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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

export default function ChatbotPage() {
  const { toastError } = useToast();

  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [monitorId, setMonitorId] = useState("");

  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [activeArtifact, setActiveArtifact] = useState<
    ChatMessage["artifact"] | null
  >(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Reset the active conversation/thread when the monitor changes. Adjusting
  // state during render (guarded by comparing against the previous prop) is
  // the pattern React recommends instead of doing it in an effect.
  const [prevMonitorId, setPrevMonitorId] = useState(monitorId);
  if (monitorId !== prevMonitorId) {
    setPrevMonitorId(monitorId);
    setConversationId(null);
    setMessages([]);
    setActiveArtifact(null);
  }

  useEffect(() => {
    monitorsApi
      .list()
      .then(setMonitors)
      .catch(() => toastError("Error al cargar monitores"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!monitorId) return;
    chatApi
      .listConversations(monitorId)
      .then(setConversations)
      .catch(() => toastError("Error al cargar conversaciones"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [monitorId]);

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
    if (!monitorId) return;
    try {
      const conv = await chatApi.createConversation(monitorId);
      setConversations((prev) => [conv, ...prev]);
      setConversationId(conv.id);
      setMessages([]);
    } catch {
      toastError("Error al crear la conversación");
    }
  }

  async function send(e: React.FormEvent) {
    e.preventDefault();
    if (!input.trim()) return;

    let activeId = conversationId;
    if (!activeId) {
      try {
        const conv = await chatApi.createConversation(monitorId);
        setConversations((prev) => [conv, ...prev]);
        setConversationId(conv.id);
        activeId = conv.id;
      } catch {
        toastError("Error al crear la conversación");
        return;
      }
    }

    const question = input;
    setInput("");
    setMessages((prev) => [
      ...prev,
      {
        id: `local-${Date.now()}`,
        conversationId: activeId!,
        role: "user",
        content: question,
        createdAt: new Date().toISOString(),
      },
    ]);
    setSending(true);
    try {
      const answer = await chatApi.ask(activeId!, question);
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
        <div className="max-w-xs space-y-1.5">
          <Label>Monitor</Label>
          <select
            value={monitorId}
            onChange={(e) => setMonitorId(e.target.value)}
            className="w-full h-9 rounded-md border bg-background px-3 text-sm"
          >
            <option value="">Seleccioná un monitor</option>
            {monitors.map((m) => (
              <option key={m.id} value={m.id}>
                {m.name}
              </option>
            ))}
          </select>
        </div>

        {monitorId && (
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
                onClick={newConversation}
              >
                <Plus className="h-4 w-4" /> Nueva conversación
              </Button>
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
              <CardContent className="flex-1 overflow-y-auto p-4 space-y-3">
                {messages.length === 0 && (
                  <p className="text-sm text-muted-foreground flex items-center gap-2">
                    <Bot className="h-4 w-4" /> Preguntame algo sobre este
                    monitor.
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
                  placeholder="Preguntá algo sobre los datos de este monitor"
                  disabled={sending}
                />
                <Button type="submit" disabled={sending || !input.trim()}>
                  <Send className="h-4 w-4" />
                </Button>
              </form>
            </Card>

            {activeArtifact && <ArtifactCanvas artifact={activeArtifact} />}
          </div>
        )}
      </div>
    </>
  );
}
