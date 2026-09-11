"use client";

import { useState } from "react";
import { Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import type { Monitor } from "@/lib/types";

interface AddMonitorButtonProps {
  /** Todos los monitores disponibles (ya sin los de borrado lógico). */
  monitors: Monitor[];
  /** Los que la conversación ya tiene. */
  selected: string[];
  onAdd: (monitorId: string) => void;
  /** Tope del backend (models.MaxMonitorsPerConversation). */
  max: number;
  disabled?: boolean;
}

/**
 * Agrega un monitor a una conversación ya abierta.
 *
 * Solo ofrece los monitores que la conversación NO tiene: el backend
 * trata agregar uno repetido como idempotente, pero esa idempotencia es
 * la red de seguridad, no el camino feliz — desde acá nunca se ejerce.
 *
 * No existe la operación inversa a propósito: el historial ya referenció
 * los datos de ese monitor, y quitarlo dejaría mensajes anteriores
 * hablando de algo que el asistente ya no puede ver (ver el spec).
 */
export function AddMonitorButton({
  monitors,
  selected,
  onAdd,
  max,
  disabled,
}: AddMonitorButtonProps) {
  const [open, setOpen] = useState(false);

  const disponibles = monitors.filter((m) => !selected.includes(m.id));
  const atLimit = selected.length >= max;
  // El motivo se muestra, no se esconde el botón: un control que
  // desaparece sin explicación se lee como un bug.
  const motivo = atLimit
    ? `Ya son ${max} monitores, el máximo por conversación`
    : disponibles.length === 0
      ? "No quedan monitores para agregar"
      : "Agregar un monitor a esta conversación";
  const bloqueado = disabled || atLimit || disponibles.length === 0;

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        disabled={bloqueado}
        title={motivo}
        aria-label={motivo}
        aria-expanded={open}
        className={cn(
          "flex h-6 w-6 items-center justify-center rounded-full border border-dashed text-muted-foreground",
          bloqueado
            ? "cursor-not-allowed opacity-50"
            : "hover:bg-accent hover:text-accent-foreground",
        )}
      >
        <Plus className="h-3.5 w-3.5" />
      </button>

      {open && !bloqueado && (
        <>
          {/* Capa de cierre: un clic fuera cierra el desplegable. */}
          <div
            className="fixed inset-0 z-10"
            onClick={() => setOpen(false)}
            aria-hidden="true"
          />
          <div className="absolute left-0 top-7 z-20 max-h-56 w-56 overflow-y-auto rounded-md border bg-background shadow-md">
            {disponibles.map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => {
                  onAdd(m.id);
                  setOpen(false);
                }}
                className="block w-full truncate px-3 py-2 text-left text-sm hover:bg-accent"
              >
                {m.name}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
