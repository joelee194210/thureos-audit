"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import type { SourceType, APIMode, APIAuthType } from "@/lib/types";

// Delimitadores comunes en archivos de logs y exportes bancarios — evita que
// el usuario tenga que adivinar el caracter exacto o pegarlo a mano.
const DELIMITER_PRESETS = [
  { value: ",", label: "Coma ( , )" },
  { value: ";", label: "Punto y coma ( ; )" },
  { value: "|", label: "Pipe ( | )" },
  { value: "\t", label: "Tabulador" },
  { value: " ", label: "Espacio" },
];

const DELIMITER_DEFAULT = "__default__";
const DELIMITER_CUSTOM = "__custom__";

function isPresetDelimiter(delimiter: string) {
  return DELIMITER_PRESETS.some((p) => p.value === delimiter);
}

// El valor mostrado en el Select no se puede derivar solo de `delimiter`:
// "" es ambiguo entre "por defecto" y "elegí Otro pero no he tecleado nada
// todavía". customMode desambigua ese segundo caso.
function delimiterSelectValue(delimiter: string, customMode: boolean) {
  if (delimiter) return isPresetDelimiter(delimiter) ? delimiter : DELIMITER_CUSTOM;
  return customMode ? DELIMITER_CUSTOM : DELIMITER_DEFAULT;
}

export interface SourceConfigValues {
  delimiter: string;
  hasHeaderRow: boolean;
  sheetName: string;
  rootPath: string;
  apiMode: APIMode;
  pullUrl: string;
  pullMethod: string;
  pullAuthType: APIAuthType;
  pullAuthHeaderName: string;
  pullAuthValue: string;
  pullIntervalMinutes: string;
}

interface SourceConfigFieldsProps {
  sourceType: SourceType;
  values: SourceConfigValues;
  onChange: (patch: Partial<SourceConfigValues>) => void;
  /** En edición no se puede volver a elegir modo push/pull ni ver el token — ya existe. */
  editing?: boolean;
}

export function SourceConfigFields({ sourceType, values, onChange, editing }: SourceConfigFieldsProps) {
  const [customMode, setCustomMode] = useState(() => !!values.delimiter && !isPresetDelimiter(values.delimiter));
  const isCustomDelimiter = delimiterSelectValue(values.delimiter, customMode) === DELIMITER_CUSTOM;

  return (
    <>
      {(sourceType === "csv" || sourceType === "txt") && (
        <div className="space-y-3 rounded-md border p-3">
          <div className="space-y-2">
            <Label>Delimitador</Label>
            <Select
              value={delimiterSelectValue(values.delimiter, customMode)}
              onValueChange={(v) => {
                if (v === DELIMITER_DEFAULT) { setCustomMode(false); onChange({ delimiter: "" }); }
                else if (v === DELIMITER_CUSTOM) { setCustomMode(true); if (!isCustomDelimiter) onChange({ delimiter: "" }); }
                else { setCustomMode(false); onChange({ delimiter: v }); }
              }}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value={DELIMITER_DEFAULT}>
                  {sourceType === "txt" ? "Tab (por defecto)" : "Coma (por defecto)"}
                </SelectItem>
                {DELIMITER_PRESETS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                ))}
                <SelectItem value={DELIMITER_CUSTOM}>Otro…</SelectItem>
              </SelectContent>
            </Select>
            {isCustomDelimiter && (
              <Input
                value={values.delimiter}
                onChange={(e) => onChange({ delimiter: e.target.value.slice(0, 1) })}
                placeholder="Un solo caracter, ej. §"
                maxLength={1}
              />
            )}
          </div>
          <div className="flex items-center justify-between">
            <Label>Primera fila es encabezado</Label>
            <Switch
              checked={values.hasHeaderRow}
              onCheckedChange={(v) => onChange({ hasHeaderRow: v })}
            />
          </div>
        </div>
      )}

      {sourceType === "excel" && (
        <div className="space-y-2 rounded-md border p-3">
          <Label>Nombre de la hoja</Label>
          <Input
            value={values.sheetName}
            onChange={(e) => onChange({ sheetName: e.target.value })}
            placeholder="Primera hoja (por defecto)"
          />
        </div>
      )}

      {sourceType === "json" && (
        <div className="space-y-2 rounded-md border p-3">
          <Label>Ruta raíz (opcional)</Label>
          <Input
            value={values.rootPath}
            onChange={(e) => onChange({ rootPath: e.target.value })}
            placeholder="ej. data.records — vacío usa la raíz del JSON"
          />
        </div>
      )}

      {sourceType === "api" && (
        <div className="space-y-3 rounded-md border p-3">
          {editing ? (
            <div className="space-y-2">
              <Label>Modo</Label>
              <p className="text-sm text-muted-foreground">
                {values.apiMode === "push" ? "Push — el sistema externo nos envía datos" : "Pull — consultamos una API externa por horario"}
                {" "}(no se puede cambiar después de crear el monitor)
              </p>
            </div>
          ) : (
            <div className="space-y-2">
              <Label>Modo</Label>
              <Select
                value={values.apiMode}
                onValueChange={(v) => onChange({ apiMode: v as APIMode })}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="push">Push — el sistema externo nos envía datos</SelectItem>
                  <SelectItem value="pull">Pull — consultamos una API externa por horario</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="space-y-2">
            <Label>Ruta raíz (opcional)</Label>
            <Input
              value={values.rootPath}
              onChange={(e) => onChange({ rootPath: e.target.value })}
              placeholder="ej. data.records — si la API envuelve el array en un campo"
            />
          </div>

          {values.apiMode === "push" ? (
            !editing && (
              <p className="text-xs text-muted-foreground">
                Al crear el monitor se genera una URL y un token — se muestran una sola vez.
              </p>
            )
          ) : (
            <>
              <div className="space-y-2">
                <Label>URL a consultar</Label>
                <Input
                  value={values.pullUrl}
                  onChange={(e) => onChange({ pullUrl: e.target.value })}
                  placeholder="https://api.ejemplo.com/transacciones"
                  required
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-2">
                  <Label>Método</Label>
                  <Select
                    value={values.pullMethod}
                    onValueChange={(v) => onChange({ pullMethod: v })}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="GET">GET</SelectItem>
                      <SelectItem value="POST">POST</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-2">
                  <Label>Cada (minutos)</Label>
                  <Input
                    type="number"
                    min={1}
                    value={values.pullIntervalMinutes}
                    onChange={(e) => onChange({ pullIntervalMinutes: e.target.value })}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label>Autenticación</Label>
                <Select
                  value={values.pullAuthType}
                  onValueChange={(v) => onChange({ pullAuthType: v as APIAuthType })}
                >
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">Ninguna</SelectItem>
                    <SelectItem value="api_key_header">API key (header)</SelectItem>
                    <SelectItem value="bearer">Bearer token</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {values.pullAuthType === "api_key_header" && (
                <div className="space-y-2">
                  <Label>Nombre del header</Label>
                  <Input
                    value={values.pullAuthHeaderName}
                    onChange={(e) => onChange({ pullAuthHeaderName: e.target.value })}
                    placeholder="ej. X-API-Key"
                  />
                </div>
              )}
              {values.pullAuthType !== "none" && (
                <div className="space-y-2">
                  <Label>{values.pullAuthType === "bearer" ? "Token" : "Valor de la API key"}</Label>
                  <Input
                    type="password"
                    value={values.pullAuthValue}
                    onChange={(e) => onChange({ pullAuthValue: e.target.value })}
                    placeholder={editing ? "Dejar en blanco para no cambiarlo" : undefined}
                  />
                </div>
              )}
            </>
          )}
        </div>
      )}
    </>
  );
}

// alwaysInclude: al editar, un campo opcional vacío significa "bórralo", no
// "no lo toques" — así que el objeto de config debe enviarse igual (con el
// campo ausente) en vez de omitirse entero, que dejaría el valor anterior
// intacto. Al crear no hay valor anterior que preservar, así que se omite.
export function sourceConfigValuesToInput(
  sourceType: SourceType,
  values: SourceConfigValues,
  opts?: { alwaysInclude?: boolean }
) {
  if (sourceType === "csv" || sourceType === "txt") {
    return {
      delimiter: values.delimiter || undefined,
      hasHeaderRow: values.hasHeaderRow,
    };
  }
  if (sourceType === "excel") {
    if (values.sheetName) return { sheetName: values.sheetName };
    return opts?.alwaysInclude ? {} : undefined;
  }
  if (sourceType === "json") {
    if (values.rootPath) return { rootPath: values.rootPath };
    return opts?.alwaysInclude ? {} : undefined;
  }
  if (sourceType === "api") {
    return values.apiMode === "pull"
      ? {
          mode: "pull" as const,
          pullUrl: values.pullUrl,
          pullMethod: values.pullMethod,
          pullAuthType: values.pullAuthType,
          pullAuthHeaderName: values.pullAuthHeaderName || undefined,
          pullAuthValue: values.pullAuthValue || undefined,
          pullIntervalMinutes: Number(values.pullIntervalMinutes) || 60,
          rootPath: values.rootPath || undefined,
        }
      : { mode: "push" as const, rootPath: values.rootPath || undefined };
  }
  return undefined;
}
