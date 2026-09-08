"use client";

import { useCallback, useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/lib/use-toast";
import { useAuthStore } from "@/stores/auth-store";
import { redFlagsApi, type CaseNote } from "@/lib/api/red-flags";
import { screeningApi, type ScreeningResult } from "@/lib/api/screening";
import {
  CASE_LABELS,
  type RedFlag,
  type RedFlagDisposition,
  type RedFlagStatus,
} from "@/lib/types";
import { formatDate, cn } from "@/lib/utils";
import { RISK_CLASSES } from "@/lib/semantic-colors";
import { useTabStore } from "@/stores/tab-store";

// Transiciones legales desde cada estado activo (espejo de la máquina del
// backend — la autoridad sigue siendo ValidTransition en el servidor).
const NEXT_STATUSES: Record<string, RedFlagStatus[]> = {
  new: ["acknowledged", "resolved", "dismissed"],
  acknowledged: ["escalated", "resolved", "dismissed"],
  escalated: ["acknowledged", "resolved", "dismissed"],
};

const DISPOSITIONS: { value: RedFlagDisposition; label: string }[] = [
  { value: "false_positive", label: "Falso positivo" },
  { value: "confirmed_ros", label: "Confirmado — reportar ROS" },
  { value: "no_action", label: "Confirmado — sin acción" },
];

export function RedFlagTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
  const { toastError, toastSuccess } = useToast();
  const user = useAuthStore((s) => s.user);
  const [rf, setRf] = useState<RedFlag | null>(null);
  const [notes, setNotes] = useState<CaseNote[]>([]);
  const [screenings, setScreenings] = useState<ScreeningResult[]>([]);
  const [noteText, setNoteText] = useState("");
  const [closeStatus, setCloseStatus] = useState("");
  const [disposition, setDisposition] = useState("");
  const [busy, setBusy] = useState(false);
  const [dismissingId, setDismissingId] = useState<string | null>(null);
  const [dismissNotes, setDismissNotes] = useState("");
  const [dismissing, setDismissing] = useState(false);

  const canAct = user != null && user.role !== "viewer";

  const load = useCallback(async () => {
    try {
      const [flag, caseNotes, screeningResults] = await Promise.all([
        redFlagsApi.get(id),
        redFlagsApi.listNotes(id),
        screeningApi.getByRedFlag(id),
      ]);
      setRf(flag);
      setNotes(caseNotes);
      setScreenings(screeningResults);
    } catch {
      toastError("Error al cargar el caso");
    }
  }, [id]);

  useEffect(() => {
    // load() es async: todo setState ocurre después de un await, nunca
    // sincrónico dentro del efecto — la regla no aplica a este caso.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);

  useEffect(() => {
    if (rf?.ruleName) {
      updateTabLabel(tabId, rf.ruleName);
      registerEntityLabel(id, rf.ruleName);
    }
  }, [rf?.ruleName, tabId, id, updateTabLabel, registerEntityLabel]);

  if (!rf) {
    return (
      <>
        <Header title="Caso" />
        <div className="p-6 text-sm text-ink-muted">Cargando…</div>
      </>
    );
  }

  const closed = rf.status === "resolved" || rf.status === "dismissed";
  const nexts = NEXT_STATUSES[rf.status] ?? [];
  const slaVencido =
    rf.slaDueAt != null && !closed && new Date(rf.slaDueAt) < new Date();

  async function run(fn: () => Promise<unknown>, ok: string) {
    setBusy(true);
    try {
      await fn();
      toastSuccess(ok);
      await load();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error en la operación");
    } finally {
      setBusy(false);
    }
  }

  async function dismissScreening(
    screeningId: string,
    status: "dismissed" | "false_positive",
  ) {
    if (!dismissNotes.trim()) {
      toastError("La nota es obligatoria para descartar un screening");
      return;
    }
    setDismissing(true);
    try {
      await screeningApi.dismiss(screeningId, { status, notes: dismissNotes });
      toastSuccess("Screening descartado");
      setDismissingId(null);
      setDismissNotes("");
      await load();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al descartar");
    } finally {
      setDismissing(false);
    }
  }

  return (
    <>
      <Header title={`Caso — ${rf.ruleName}`} />
      <div className="p-6 space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "text-xs font-semibold px-2 py-0.5 rounded",
              RISK_CLASSES[rf.severity] ?? "",
            )}
          >
            {rf.severity}
          </span>
          <span className="text-sm font-medium">
            {CASE_LABELS[rf.status] ?? rf.status}
          </span>
          {rf.assigneeId != null && (
            <span className="text-xs text-ink-muted">
              Asignado a {rf.assigneeId === user?.id ? "mí" : rf.assigneeId}
            </span>
          )}
          {rf.slaDueAt != null && (
            <span
              className={cn(
                "text-xs",
                slaVencido ? "text-danger-fg font-semibold" : "text-ink-muted",
              )}
            >
              SLA: {formatDate(rf.slaDueAt)}
              {slaVencido ? " (vencido)" : ""}
            </span>
          )}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Datos de la alerta */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Alerta</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2 text-sm">
              <div>
                <span className="text-ink-muted">Monitor:</span>{" "}
                {rf.monitorName}
              </div>
              <div>
                <span className="text-ink-muted">Mensaje:</span> {rf.message}
              </div>
              <div>
                <span className="text-ink-muted">Coincidencias:</span>{" "}
                {rf.matchCount}
              </div>
              <div>
                <span className="text-ink-muted">Creada:</span>{" "}
                {formatDate(rf.createdAt)}
              </div>
              {rf.disposition != null && (
                <div>
                  <span className="text-ink-muted">Disposición:</span>{" "}
                  {DISPOSITIONS.find((d) => d.value === rf.disposition)
                    ?.label ?? rf.disposition}
                </div>
              )}
              {rf.matchedData != null && (
                <pre className="mt-2 max-h-64 overflow-auto rounded-md bg-muted p-3 text-xs font-mono">
                  {JSON.stringify(rf.matchedData, null, 2)}
                </pre>
              )}
            </CardContent>
          </Card>

          {/* Acciones del caso */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Investigación</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {canAct && !closed && (
                <div className="space-y-3">
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={busy || rf.status !== "new"}
                      onClick={() =>
                        run(
                          () => redFlagsApi.assignCase(rf.id, "me"),
                          "Caso asignado",
                        )
                      }
                    >
                      Tomar caso
                    </Button>
                    {nexts
                      .filter((s) => s !== "resolved" && s !== "dismissed")
                      .map((s) => (
                        <Button
                          key={s}
                          variant="outline"
                          size="sm"
                          disabled={busy}
                          onClick={() =>
                            run(
                              () => redFlagsApi.transitionCase(rf.id, s),
                              "Caso en " + s,
                            )
                          }
                        >
                          {s === "escalated" ? "Escalar" : "Reanudar"}
                        </Button>
                      ))}
                  </div>

                  {nexts.some(
                    (s) => s === "resolved" || s === "dismissed",
                  ) && (
                    <div className="space-y-2 rounded-md border p-3">
                      <Label>Cerrar con disposición</Label>
                      <div className="flex gap-2">
                        <Select
                          value={closeStatus}
                          onValueChange={setCloseStatus}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Estado final" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="resolved">Resuelta</SelectItem>
                            <SelectItem value="dismissed">
                              Descartada
                            </SelectItem>
                          </SelectContent>
                        </Select>
                        <Select
                          value={disposition}
                          onValueChange={setDisposition}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Disposición" />
                          </SelectTrigger>
                          <SelectContent>
                            {DISPOSITIONS.map((d) => (
                              <SelectItem key={d.value} value={d.value}>
                                {d.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          size="sm"
                          disabled={
                            busy || !closeStatus || !disposition
                          }
                          onClick={() =>
                            run(
                              () =>
                                redFlagsApi.transitionCase(
                                  rf.id,
                                  closeStatus,
                                  disposition,
                                ),
                              "Caso cerrado",
                            )
                          }
                        >
                          Cerrar
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )}

              {closed && (
                <p className="text-sm text-ink-muted">
                  Caso cerrado — timeline congelado para auditoría.
                </p>
              )}

              {/* Timeline de notas */}
              <div className="space-y-2">
                <Label>Timeline de investigación</Label>
                <div className="space-y-2 max-h-72 overflow-y-auto">
                  {notes.map((n) => (
                    <div key={n.id} className="rounded-md border p-2 text-sm">
                      <div className="flex justify-between text-xs text-ink-muted">
                        <span>{n.authorName}</span>
                        <span>{formatDate(n.createdAt)}</span>
                      </div>
                      <p className="mt-1 whitespace-pre-wrap">{n.text}</p>
                    </div>
                  ))}
                  {notes.length === 0 && (
                    <p className="text-xs text-ink-muted">Sin notas aún.</p>
                  )}
                </div>
                {canAct && !closed && (
                  <div className="flex gap-2">
                    <Input
                      value={noteText}
                      onChange={(e) => setNoteText(e.target.value)}
                      placeholder="Agregar nota de investigación…"
                    />
                    <Button
                      size="sm"
                      disabled={busy || !noteText.trim()}
                      onClick={() =>
                        run(async () => {
                          await redFlagsApi.addNote(rf.id, noteText.trim());
                          setNoteText("");
                        }, "Nota agregada")
                      }
                    >
                      Agregar
                    </Button>
                  </div>
                )}
              </div>

              {screenings.length > 0 && (
                <div className="mt-4 space-y-2">
                  <h3 className="text-sm font-medium">Screening de sanciones</h3>
                  {screenings.map((s) => {
                    const open = s.status === "match" || s.status === "review";
                    return (
                      <div key={s.id} className="rounded-md border p-3 text-xs space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="font-medium">{s.field}: {s.query}</span>
                          <span
                            className={cn(
                              "rounded px-2 py-0.5 text-[10px] font-medium",
                              s.status === "clear" && "bg-success-bg text-success-fg",
                              open && "bg-danger-bg text-danger-fg",
                              (s.status === "dismissed" || s.status === "false_positive") &&
                                "bg-muted text-muted-foreground",
                            )}
                          >
                            {s.status}
                          </span>
                        </div>
                        {(s.matches || []).slice(0, 3).map((m, i) => (
                          <p key={i} className="text-muted-foreground">
                            {m.name} — {m.sourceList} ({m.score.toFixed(2)})
                          </p>
                        ))}

                        {open && dismissingId !== s.id && canAct && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="h-6 text-[11px]"
                            onClick={() => {
                              setDismissingId(s.id);
                              setDismissNotes("");
                            }}
                          >
                            Descartar
                          </Button>
                        )}

                        {dismissingId === s.id && (
                          <div className="space-y-2 pt-1">
                            <Input
                              value={dismissNotes}
                              onChange={(e) => setDismissNotes(e.target.value)}
                              placeholder="Nota obligatoria: por qué se descarta"
                              className="text-xs h-7"
                            />
                            <div className="flex gap-2">
                              <Button
                                size="sm"
                                className="h-6 text-[11px]"
                                disabled={dismissing}
                                onClick={() => dismissScreening(s.id, "false_positive")}
                              >
                                Falso positivo
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                className="h-6 text-[11px]"
                                disabled={dismissing}
                                onClick={() => dismissScreening(s.id, "dismissed")}
                              >
                                Descartar
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                className="h-6 text-[11px]"
                                onClick={() => setDismissingId(null)}
                              >
                                Cancelar
                              </Button>
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}

