"use client";

import { useState } from "react";
import { Check, Search, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import type { Monitor } from "@/lib/types";

interface MonitorMultiSelectProps {
  monitors: Monitor[];
  selected: string[];
  onChange: (ids: string[]) => void;
  /** Tope del backend (models.MaxMonitorsPerConversation). */
  max: number;
}

/**
 * Selector de los monitores que va a abarcar una conversación. El
 * conjunto elegido acá es la allowlist que el backend usa para resolver
 * los alias del LLM, así que el tope se respeta también del lado del
 * servidor: esto es comodidad, no seguridad.
 */
export function MonitorMultiSelect({
  monitors,
  selected,
  onChange,
  max,
}: MonitorMultiSelectProps) {
  const [query, setQuery] = useState("");

  const visible = monitors.filter((m) =>
    m.name.toLowerCase().includes(query.trim().toLowerCase()),
  );
  const atLimit = selected.length >= max;

  function toggle(id: string) {
    if (selected.includes(id)) {
      onChange(selected.filter((s) => s !== id));
    } else if (!atLimit) {
      onChange([...selected, id]);
    }
  }

  return (
    <div className="space-y-2">
      {selected.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {selected.map((id) => {
            const monitor = monitors.find((m) => m.id === id);
            return (
              <span
                key={id}
                className="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
              >
                {monitor?.name ?? id}
                <button
                  type="button"
                  onClick={() => toggle(id)}
                  aria-label={`Quitar ${monitor?.name ?? id}`}
                  className="rounded-full hover:bg-background/50"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            );
          })}
        </div>
      )}

      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Buscar monitor…"
          className="pl-8"
        />
      </div>

      <div className="max-h-56 overflow-y-auto rounded-md border">
        {visible.length === 0 && (
          <p className="p-3 text-sm text-muted-foreground">
            Ningún monitor coincide con la búsqueda.
          </p>
        )}
        {visible.map((m) => {
          const isSelected = selected.includes(m.id);
          const disabled = !isSelected && atLimit;
          return (
            <button
              key={m.id}
              type="button"
              disabled={disabled}
              onClick={() => toggle(m.id)}
              className={cn(
                "flex w-full items-center gap-2 px-3 py-2 text-left text-sm",
                isSelected ? "bg-accent/50" : "hover:bg-accent/30",
                disabled && "cursor-not-allowed opacity-50",
              )}
            >
              <span
                className={cn(
                  "flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border",
                  isSelected && "bg-primary text-primary-foreground",
                )}
              >
                {isSelected && <Check className="h-3 w-3" />}
              </span>
              <span className="truncate">{m.name}</span>
            </button>
          );
        })}
      </div>

      <p className="text-xs text-muted-foreground">
        {selected.length} de {max} monitores
        {atLimit && " — llegaste al tope"}
      </p>
    </div>
  );
}
