"use client";

import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Plus, Trash2 } from "lucide-react";
import { ConditionRows } from "@/components/rules/condition-rows";
import type { Condition, SchemaField, VelocityCondition } from "@/lib/types";

/** Los mismos formatos que acepta el backend para el gap máximo. */
const GAP_OPTIONS = [
  { value: "10s", label: "10 segundos" },
  { value: "30s", label: "30 segundos" },
  { value: "35s", label: "35 segundos" },
  { value: "60s", label: "60 segundos" },
  { value: "5min", label: "5 minutos" },
  { value: "15min", label: "15 minutos" },
  { value: "1h", label: "1 hora" },
];

export function nuevaCondicionDeVelocidad(
  schema: SchemaField[],
): VelocityCondition {
  const primerFecha = schema.find((f) => f.type === "date");
  return {
    timeField: primerFecha?.name ?? "",
    maxGap: "60s",
    groupBy: "",
    minEvents: 2,
    filter: [],
  };
}

type Props = {
  conditions: VelocityCondition[];
  schema: SchemaField[];
  onChange: (conditions: VelocityCondition[]) => void;
  renderFilterValue?: (
    cond: Condition,
    onValueChange: (v: string) => void,
  ) => ReactNode | null;
};

/**
 * Editor de condiciones de velocidad: detectan dos transacciones consecutivas
 * de la misma entidad separadas por menos del gap configurado.
 *
 * El campo de tiempo solo lista campos de tipo date, porque el motor mide
 * diferencias de tiempo y rechaza cualquier otro tipo al guardar. Si el
 * monitor no tiene ninguno, se explica cómo obtenerlo en vez de ofrecer una
 * lista vacía.
 */
export function VelocityConditionEditor({
  conditions,
  schema,
  onChange,
  renderFilterValue,
}: Props) {
  const camposFecha = schema.filter((f) => f.type === "date");

  function update(i: number, patch: Partial<VelocityCondition>) {
    onChange(conditions.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <Label className="text-sm">Velocidad entre transacciones</Label>
          <p className="text-xs text-muted-foreground">
            Detecta dos transacciones consecutivas de la misma entidad
            separadas por menos del tiempo configurado.
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() =>
            onChange([...conditions, nuevaCondicionDeVelocidad(schema)])
          }
          disabled={camposFecha.length === 0}
        >
          <Plus className="mr-1 h-3 w-3" />
          Agregar
        </Button>
      </div>

      {camposFecha.length === 0 && (
        <p className="rounded-md border border-dashed p-3 text-xs text-muted-foreground">
          Este monitor no tiene ningún campo de tipo fecha, así que no se puede
          medir el tiempo entre transacciones. Si la fecha y la hora vienen en
          columnas numéricas separadas, configurá el timestamp derivado en la
          pestaña Esquema del monitor.
        </p>
      )}

      {conditions.map((cond, i) => (
        <div key={i} className="space-y-3 rounded-md border p-3">
          <div className="grid grid-cols-3 gap-2">
            <div className="space-y-1">
              <Label className="text-xs">Campo de tiempo</Label>
              <Select
                value={cond.timeField}
                onValueChange={(v) => update(i, { timeField: v })}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Campo fecha" />
                </SelectTrigger>
                <SelectContent>
                  {camposFecha.map((f) => (
                    <SelectItem key={f.name} value={f.name}>
                      {f.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label className="text-xs">Máximo entre una y otra</Label>
              <Select
                value={cond.maxGap}
                onValueChange={(v) => update(i, { maxGap: v })}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {GAP_OPTIONS.map((g) => (
                    <SelectItem key={g.value} value={g.value}>
                      {g.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label className="text-xs">Agrupar por</Label>
              <Select
                value={cond.groupBy}
                onValueChange={(v) => update(i, { groupBy: v })}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="Entidad" />
                </SelectTrigger>
                <SelectContent>
                  {schema.map((f) => (
                    <SelectItem key={f.name} value={f.name}>
                      {f.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="space-y-1">
            <Label className="text-xs">
              Filtrar antes de comparar (opcional)
            </Label>
            <ConditionRows
              conditions={cond.filter ?? []}
              schema={schema}
              onChange={(f) => update(i, { filter: f })}
              renderValue={renderFilterValue}
              emptyLabel="Sin filtro: se comparan todas las transacciones."
            />
          </div>

          <div className="flex justify-end">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-7 text-xs text-destructive"
              onClick={() => onChange(conditions.filter((_, idx) => idx !== i))}
            >
              <Trash2 className="mr-1 h-3 w-3" />
              Quitar condición
            </Button>
          </div>
        </div>
      ))}
    </div>
  );
}
