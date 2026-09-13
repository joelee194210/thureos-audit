"use client";

import { useEffect, useState } from "react";
import { BarChart3, Code2, Table2, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { ChatArtifact, ChatArtifactType } from "@/lib/api/chat";
import { artifactsApi } from "@/lib/api/artifacts";
import { formatRanAt } from "@/lib/artifact-report";
import { useToast } from "@/lib/use-toast";
import type { Monitor } from "@/lib/types";

const TYPE_ICON: Record<ChatArtifactType, typeof BarChart3> = {
  chart: BarChart3,
  table: Table2,
  custom: Code2,
};

interface ArtifactLibraryProps {
  monitors: Monitor[];
  /** El backend no manda `monitorsLoaded`: mientras la lista no llegó, un
   *  id que no resuelve no es evidencia de que el monitor esté borrado. */
  monitorsLoaded: boolean;
  onOpen: (artifact: ChatArtifact) => void;
}

/**
 * Biblioteca de artefactos guardados. Abrir uno lo manda directo al canvas
 * (que lo re-ejecuta contra datos actuales) sin necesidad de una
 * conversación activa — ese es el punto de poder invocarlos directamente.
 */
export function ArtifactLibrary({ monitors, monitorsLoaded, onOpen }: ArtifactLibraryProps) {
  const { toastError } = useToast();
  const [artifacts, setArtifacts] = useState<ChatArtifact[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  useEffect(() => {
    artifactsApi
      .list()
      .then((data) => {
        setArtifacts(data);
        setLoaded(true);
      })
      .catch(() => toastError("Error al cargar los artefactos guardados"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function monitorLabel(id: string): string {
    const found = monitors.find((m) => m.id === id);
    if (found) return found.name;
    return monitorsLoaded ? "Monitor eliminado" : "Cargando…";
  }

  async function confirmDelete(id: string) {
    try {
      await artifactsApi.remove(id);
      setArtifacts((prev) => prev.filter((a) => a.id !== id));
    } catch {
      toastError("Error al borrar el artefacto");
    } finally {
      setDeletingId(null);
    }
  }

  if (loaded && artifacts.length === 0) {
    return (
      <p className="px-2 py-3 text-xs text-muted-foreground">
        Todavía no guardaste ningún artefacto. Cuando el asistente genere un
        gráfico o una tabla, guardalo para volver a abrirlo cuando quieras con
        datos actualizados.
      </p>
    );
  }

  return (
    <div className="space-y-1">
      {artifacts.map((a) => {
        if (!a.id) return null;
        const id = a.id;
        const Icon = TYPE_ICON[a.type];

        if (deletingId === id) {
          return (
            <div
              key={id}
              className="flex items-center gap-1 rounded-md bg-destructive/10 px-2 py-1.5 text-xs"
            >
              <span className="flex-1 truncate text-muted-foreground">¿Borrar?</span>
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-1.5 text-destructive hover:text-destructive"
                onClick={() => confirmDelete(id)}
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
          );
        }

        return (
          <div key={id} className="group flex items-center">
            <button
              onClick={() => onOpen(a)}
              className="flex min-w-0 flex-1 items-start gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent/50"
            >
              <Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                <span className="block truncate">{a.savedName ?? a.title}</span>
                {(a.monitorIds?.length ?? 0) > 0 && (
                  <span className="mt-0.5 flex flex-wrap gap-1">
                    {a.monitorIds!.map((mid) => (
                      <span
                        key={mid}
                        className="rounded-full bg-accent px-1.5 py-0 text-[10px] text-accent-foreground"
                      >
                        {monitorLabel(mid)}
                      </span>
                    ))}
                  </span>
                )}
                <span className="mt-0.5 block truncate font-mono text-[11px] text-muted-foreground">
                  {formatRanAt(a.ranAt)}
                </span>
              </span>
            </button>
            <button
              onClick={(e) => {
                e.stopPropagation();
                setDeletingId(id);
              }}
              className="shrink-0 rounded-sm p-1 text-muted-foreground opacity-0 hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100"
              aria-label={`Borrar artefacto ${a.savedName ?? a.title}`}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
