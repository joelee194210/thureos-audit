"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import type {
  DerivedDateFormat,
  DerivedTimeFormat,
  DerivedTimestampConfig,
  Monitor,
  SchemaField,
} from "@/lib/types";

/**
 * Solo sirven como origen los campos numéricos o de texto sin decimales
 * implícitos: el backend rechaza el resto porque nunca podría construir un
 * timestamp con ellos (un campo ya tipado como date, o uno al que se le
 * dividen decimales, no parsean como YYYYMMDD/HHMMSS).
 */
function sourceFieldOptions(schema: SchemaField[]): SchemaField[] {
  return schema.filter(
    (f) =>
      (f.type === "number" || f.type === "string") && !f.impliedDecimals,
  );
}

type Props = {
  monitor: Monitor;
  canEdit: boolean;
  onSaved: (monitor: Monitor) => void;
};

/**
 * Configura el timestamp derivado de un monitor: combina una columna de fecha
 * y una de hora, ambas numéricas, en un campo de fecha real. Va como bloque
 * aparte de la lista de campos porque no edita un campo existente — toma dos
 * y produce uno nuevo.
 */
export function DerivedTimestampCard({ monitor, canEdit, onSaved }: Props) {
  const { toastError, toastSuccess } = useToast();
  const existing = monitor.derivedTimestamp;

  const [dateField, setDateField] = useState(existing?.dateField ?? "");
  const [dateFormat, setDateFormat] = useState<DerivedDateFormat>(
    existing?.dateFormat ?? "YYYYMMDD",
  );
  const [timeField, setTimeField] = useState(existing?.timeField ?? "");
  const [timeFormat, setTimeFormat] = useState<DerivedTimeFormat>(
    existing?.timeFormat ?? "HHMMSS",
  );
  const [targetName, setTargetName] = useState(
    existing?.targetName ?? "timestamp",
  );
  const [saving, setSaving] = useState(false);
  const [backfilling, setBackfilling] = useState(false);

  const options = sourceFieldOptions(monitor.schema ?? []);
  const puedeGuardar = !!dateField && !!timeField && !!targetName.trim();

  async function guardar() {
    setSaving(true);
    try {
      const cfg: DerivedTimestampConfig = {
        dateField,
        dateFormat,
        timeField,
        timeFormat,
        targetName: targetName.trim(),
      };
      const actualizado = await monitorsApi.updateDerivedTimestamp(
        monitor.id,
        cfg,
      );
      onSaved(actualizado);
      toastSuccess(`Campo "${cfg.targetName}" configurado`);
    } catch (err) {
      toastError(
        err instanceof Error
          ? err.message
          : "No se pudo configurar el timestamp derivado",
      );
    } finally {
      setSaving(false);
    }
  }

  async function limpiar() {
    setSaving(true);
    try {
      const actualizado = await monitorsApi.updateDerivedTimestamp(
        monitor.id,
        null,
      );
      onSaved(actualizado);
      setDateField("");
      setTimeField("");
      setTargetName("timestamp");
      toastSuccess("Timestamp derivado eliminado");
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "No se pudo eliminar la configuración",
      );
    } finally {
      setSaving(false);
    }
  }

  async function backfill() {
    setBackfilling(true);
    try {
      const r = await monitorsApi.backfillTimestamp(monitor.id);
      const noConstruibles =
        r.skipped > 0 ? `, ${r.skipped} sin poder construir` : "";
      toastSuccess(
        `${r.updated} registros actualizados${noConstruibles}`,
      );
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "No se pudo completar el backfill",
      );
    } finally {
      setBackfilling(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Timestamp derivado</CardTitle>
        <p className="text-xs text-muted-foreground">
          Si la fecha y la hora vienen en dos columnas numéricas separadas,
          acá se combinan en un campo de fecha real. Sin él, las reglas que
          miden tiempo entre transacciones no pueden evaluarse.
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        {options.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            Este monitor no tiene columnas numéricas o de texto que puedan
            usarse como fecha y hora.
          </p>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Columna de fecha</Label>
                <Select
                  value={dateField}
                  onValueChange={setDateField}
                  disabled={!canEdit}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Elegí una columna" />
                  </SelectTrigger>
                  <SelectContent>
                    {options.map((f) => (
                      <SelectItem key={f.name} value={f.name}>
                        {f.name}
                        {f.sample ? ` (ej. ${f.sample})` : ""}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Formato de la fecha</Label>
                <Select
                  value={dateFormat}
                  onValueChange={(v) => setDateFormat(v as DerivedDateFormat)}
                  disabled={!canEdit}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="YYYYMMDD">
                      YYYYMMDD (ej. 20260907)
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>Columna de hora</Label>
                <Select
                  value={timeField}
                  onValueChange={setTimeField}
                  disabled={!canEdit}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Elegí una columna" />
                  </SelectTrigger>
                  <SelectContent>
                    {options.map((f) => (
                      <SelectItem key={f.name} value={f.name}>
                        {f.name}
                        {f.sample ? ` (ej. ${f.sample})` : ""}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Formato de la hora</Label>
                <Select
                  value={timeFormat}
                  onValueChange={(v) => setTimeFormat(v as DerivedTimeFormat)}
                  disabled={!canEdit}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="HHMMSS">
                      HHMMSS (ej. 021517 = 02:15:17)
                    </SelectItem>
                    <SelectItem value="HHMM">HHMM (ej. 0215 = 02:15)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label>Nombre del campo nuevo</Label>
              <Input
                value={targetName}
                onChange={(e) => setTargetName(e.target.value)}
                placeholder="timestamp"
                disabled={!canEdit}
              />
              <p className="text-xs text-muted-foreground">
                Aparece como campo de tipo fecha en las reglas, el chat y los
                dashboards. No puede llamarse igual que una columna del archivo.
              </p>
            </div>

            {canEdit && (
              <div className="flex flex-wrap items-center gap-2">
                <Button onClick={guardar} disabled={!puedeGuardar || saving}>
                  {saving ? "Guardando..." : "Guardar"}
                </Button>
                {existing && (
                  <>
                    <Button
                      variant="outline"
                      onClick={backfill}
                      disabled={backfilling}
                    >
                      {backfilling
                        ? "Calculando..."
                        : "Calcular para datos ya cargados"}
                    </Button>
                    <Button
                      variant="ghost"
                      onClick={limpiar}
                      disabled={saving}
                      className="text-destructive"
                    >
                      Eliminar
                    </Button>
                  </>
                )}
              </div>
            )}

            {existing && (
              <p className="text-xs text-muted-foreground">
                Las cargas nuevas ya traen el campo. Usá &quot;Calcular para
                datos ya cargados&quot; una vez para completar los registros
                que se ingirieron antes de configurarlo.
              </p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
