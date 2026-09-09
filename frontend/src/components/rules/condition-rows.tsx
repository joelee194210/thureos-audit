"use client";

import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Plus, Trash2 } from "lucide-react";
import type { Condition, Operator, SchemaField } from "@/lib/types";

/** Mismas etiquetas que el editor de condiciones principal de la página. */
const OPERATOR_LABELS: Array<{ value: Operator; label: string }> = [
  { value: "eq", label: "= Igual" },
  { value: "neq", label: "≠ Diferente" },
  { value: "gt", label: "> Mayor" },
  { value: "lt", label: "< Menor" },
  { value: "gte", label: "≥ Mayor o igual" },
  { value: "lte", label: "≤ Menor o igual" },
  { value: "contains", label: "Contiene" },
  { value: "regex", label: "Regex" },
  { value: "in", label: "En lista" },
  { value: "not_in", label: "No en lista" },
  { value: "between", label: "Entre (min,max)" },
  { value: "starts_with", label: "Comienza con" },
  { value: "ends_with", label: "Finaliza con" },
  { value: "is_null", label: "Es nulo" },
  { value: "is_not_null", label: "No es nulo" },
];

type Props = {
  conditions: Condition[];
  schema: SchemaField[];
  onChange: (conditions: Condition[]) => void;
  /**
   * Permite a la página inyectar su propio control de valor para ciertos
   * campos (el selector de MCC, por ejemplo). Devolver null usa el input
   * de texto por defecto. Es un render-prop para no arrastrar acá las
   * dependencias de ese control.
   */
  renderValue?: (
    cond: Condition,
    onValueChange: (v: string) => void,
  ) => ReactNode | null;
  disabled?: boolean;
  addLabel?: string;
  emptyLabel?: string;
};

/**
 * Editor de una lista de condiciones (campo / operador / valor). Se usa para
 * los filtros previos de las condiciones agregadas y de velocidad, donde el
 * universo se reduce antes de agrupar.
 */
export function ConditionRows({
  conditions,
  schema,
  onChange,
  renderValue,
  disabled,
  addLabel = "Agregar filtro",
  emptyLabel = "Sin filtro: se agregan todos los registros.",
}: Props) {
  function update(i: number, patch: Partial<Condition>) {
    onChange(conditions.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  }

  function remove(i: number) {
    onChange(conditions.filter((_, idx) => idx !== i));
  }

  function add() {
    onChange([
      ...conditions,
      { field: schema[0]?.name ?? "", operator: "eq", value: "" },
    ]);
  }

  const sinValor = (op: Operator) => op === "is_null" || op === "is_not_null";

  return (
    <div className="space-y-2">
      {conditions.length === 0 && (
        <p className="text-xs text-muted-foreground">{emptyLabel}</p>
      )}

      {conditions.map((cond, i) => {
        const custom = renderValue?.(cond, (v) => update(i, { value: v }));
        return (
          <div
            key={i}
            className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2"
          >
            <Select
              value={cond.field}
              onValueChange={(v) => update(i, { field: v })}
              disabled={disabled}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue placeholder="Campo" />
              </SelectTrigger>
              <SelectContent>
                {schema.map((f) => (
                  <SelectItem key={f.name} value={f.name}>
                    {f.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Select
              value={cond.operator}
              onValueChange={(v) => update(i, { operator: v as Operator })}
              disabled={disabled}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {OPERATOR_LABELS.map((op) => (
                  <SelectItem key={op.value} value={op.value}>
                    {op.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            {sinValor(cond.operator) ? (
              <div className="flex h-8 items-center text-xs text-muted-foreground">
                sin valor
              </div>
            ) : custom ? (
              custom
            ) : (
              <Input
                className="h-8 text-xs"
                value={String(cond.value ?? "")}
                onChange={(e) => update(i, { value: e.target.value })}
                placeholder={
                  cond.operator === "between" ? "min,max" : "Valor"
                }
                disabled={disabled}
              />
            )}

            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              onClick={() => remove(i)}
              disabled={disabled}
            >
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </div>
        );
      })}

      {!disabled && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 text-xs"
          onClick={add}
          disabled={schema.length === 0}
        >
          <Plus className="mr-1 h-3 w-3" />
          {addLabel}
        </Button>
      )}
    </div>
  );
}
