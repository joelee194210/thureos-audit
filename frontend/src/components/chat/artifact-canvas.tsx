"use client";

import { useEffect, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip as RechartsTooltip,
  XAxis,
  YAxis,
} from "recharts";
import { RefreshCw, Save, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import type { ChatArtifact } from "@/lib/api/chat";
import { artifactsApi, deriveSeriesFromRows, isRerunnable } from "@/lib/api/artifacts";
import { ApiError } from "@/lib/api/client";
import { ExportMenu } from "@/components/chat/export-menu";
import { formatRanAt } from "@/lib/artifact-report";
import { cn } from "@/lib/utils";
import { classifyValue, formatValue, unionColumns } from "@/lib/format-value";
import { useToast } from "@/lib/use-toast";

const CHART_COLORS = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
  "var(--chart-7)",
  "var(--chart-8)",
];

/** id del contenedor: lo reutilizan chartToPNGDataURL y ExportMenu. */
const CONTAINER_ID = "artifact-canvas-content";

/** Filas y serie a pintar de arranque, antes de que llegue una corrida fresca. */
function initialRows(artifact: ChatArtifact): Record<string, unknown>[] {
  return artifact.cachedData ?? artifact.chartSpec?.data ?? [];
}

function initialSeries(artifact: ChatArtifact): string[] {
  // cachedData viene de una corrida real ya pivoteada; chartSpec.yKeys es
  // la lista de la ejecución ORIGINAL y ya no describe esas columnas (ver
  // deriveSeriesFromRows). chartSpec.data, en cambio, es una instantánea
  // legacy sin sources: ahí yKeys sí sigue siendo correcto.
  if (artifact.cachedData) {
    return deriveSeriesFromRows(artifact.cachedData, artifact.chartSpec?.xKey);
  }
  return artifact.chartSpec?.yKeys ?? [];
}

interface ArtifactCanvasProps {
  artifact: ChatArtifact;
  /** Nombres (no ids) de los monitores consultados, para el pie del export. */
  monitorNames?: string[];
  /** Se dispara cuando guardar/quitar cambia el artefacto en el servidor. */
  onSaved?: (artifact: ChatArtifact) => void;
}

export function ArtifactCanvas({
  artifact,
  monitorNames = [],
  onSaved,
}: ArtifactCanvasProps) {
  const { toastError, toastSuccess } = useToast();

  const [rows, setRows] = useState<Record<string, unknown>[]>(() => initialRows(artifact));
  const [series, setSeries] = useState<string[]>(() => initialSeries(artifact));
  const [ranAt, setRanAt] = useState(artifact.ranAt);
  // Arranca en `true` cuando el montaje va a disparar una corrida (ver el
  // efecto de abajo), calculado con el mismo lazy initializer que rows/
  // series: evita tener que poner un setRunning(true) síncrono al principio
  // del efecto, que React marca como setState en cascada.
  const [running, setRunning] = useState(
    () => !!artifact.id && isRerunnable(artifact),
  );
  const [staleReason, setStaleReason] = useState<string | null>(null);

  const [saved, setSaved] = useState(artifact.saved ?? false);
  const [savedName, setSavedName] = useState(artifact.savedName);
  const [savingOpen, setSavingOpen] = useState(false);
  const [nameDraft, setNameDraft] = useState("");
  const [saving, setSaving] = useState(false);

  const rerunnable = isRerunnable(artifact) && !!artifact.id;
  const refreshDisabledReason = !isRerunnable(artifact)
    ? "Este artefacto guarda una instantánea y no se puede actualizar."
    : !artifact.id
      ? "Este artefacto no quedó vinculado a datos: no se puede actualizar."
      : null;

  // Al montar: la caché/instantánea ya está pintada (los useState de arriba
  // arrancan con ella) y acá se dispara la corrida fresca si corresponde,
  // para reemplazarla en cuanto llegue. Si la corrida falla o el monitor de
  // origen desapareció (409), la caché se queda donde estaba con un aviso
  // explícito — nunca una pantalla vacía.
  //
  // No resetea estado al cambiar `artifact`: la página monta un
  // <ArtifactCanvas key={artifact.id}> nuevo por cada artefacto (ver
  // chatbot/page.tsx), así que un artefacto distinto es una instancia
  // distinta con sus propios useState — resetear "a mano" acá encima
  // duplicaría esa sincronización y el lint de React la marca como
  // setState en cascada dentro de un efecto.
  useEffect(() => {
    if (!artifact.id || !isRerunnable(artifact)) return;

    let cancelled = false;
    artifactsApi
      .run(artifact.id)
      .then((run) => {
        if (cancelled) return;
        setRows(run.data);
        setSeries(run.series);
        setRanAt(run.ranAt);
        setStaleReason(null);
      })
      .catch((err) => {
        if (cancelled) return;
        setStaleReason(
          err instanceof ApiError && err.status === 409
            ? "El monitor de origen de este artefacto ya no existe. Se muestran los datos de su última corrida."
            : "No se pudo actualizar. Se muestran los datos de su última corrida.",
        );
      })
      .finally(() => {
        if (!cancelled) setRunning(false);
      });

    return () => {
      cancelled = true;
    };
    // Se dispara solo cuando cambia el artefacto que se está mirando, no
    // en cada render: correr de nuevo por un cambio de identidad de
    // función (monitorNames/onSaved) reiniciaría la corrida sin motivo.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [artifact.id]);

  async function handleRefresh() {
    if (!artifact.id || !rerunnable || running) return;
    setRunning(true);
    try {
      const run = await artifactsApi.run(artifact.id);
      setRows(run.data);
      setSeries(run.series);
      setRanAt(run.ranAt);
      setStaleReason(null);
    } catch (err) {
      setStaleReason(
        err instanceof ApiError && err.status === 409
          ? "El monitor de origen de este artefacto ya no existe. Se muestran los datos de su última corrida."
          : "No se pudo actualizar. Se muestran los datos de su última corrida.",
      );
    } finally {
      setRunning(false);
    }
  }

  async function handleSave() {
    if (!artifact.id || !nameDraft.trim() || saving) return;
    setSaving(true);
    try {
      const updated = await artifactsApi.save(artifact.id, nameDraft.trim());
      setSaved(true);
      setSavedName(updated.savedName);
      setSavingOpen(false);
      setNameDraft("");
      onSaved?.(updated);
      toastSuccess("Artefacto guardado.");
    } catch {
      toastError("No se pudo guardar el artefacto.");
    } finally {
      setSaving(false);
    }
  }

  async function handleUnsave() {
    if (!artifact.id) return;
    try {
      await artifactsApi.unsave(artifact.id);
      setSaved(false);
      setSavedName(undefined);
      onSaved?.({ ...artifact, saved: false, savedName: undefined });
      toastSuccess("Artefacto quitado de la biblioteca.");
    } catch {
      toastError("No se pudo quitar el artefacto de la biblioteca.");
    }
  }

  const label = (key: string) => artifact.chartSpec?.labels?.[key] ?? key;

  return (
    <Card className="h-[70vh] flex flex-col">
      <CardHeader className="gap-2 pb-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="min-w-0">
            <CardTitle className="truncate text-base">
              {savedName ?? artifact.title}
            </CardTitle>
            <p className="font-mono text-xs text-muted-foreground">
              Datos al {formatRanAt(ranAt)}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefresh}
              disabled={running || !!refreshDisabledReason}
              title={refreshDisabledReason ?? "Volver a correr contra los datos actuales"}
            >
              <RefreshCw className={cn("h-4 w-4", running && "animate-spin")} />
              Actualizar
            </Button>

            {saved ? (
              <Button variant="outline" size="sm" onClick={handleUnsave} title="Quitar de la biblioteca">
                <X className="h-4 w-4" />
                {savedName}
              </Button>
            ) : savingOpen ? (
              <div className="flex items-center gap-1">
                <Input
                  autoFocus
                  value={nameDraft}
                  onChange={(e) => setNameDraft(e.target.value)}
                  placeholder="Nombre del artefacto"
                  className="h-8 w-40"
                  onKeyDown={(e) => {
                    if (e.key === "Enter") void handleSave();
                    if (e.key === "Escape") setSavingOpen(false);
                  }}
                />
                <Button size="sm" onClick={handleSave} disabled={saving || !nameDraft.trim()}>
                  Guardar
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setSavingOpen(false)}>
                  Cancelar
                </Button>
              </div>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setSavingOpen(true)}
                disabled={!artifact.id}
                title={
                  artifact.id
                    ? undefined
                    : "Este artefacto no quedó vinculado a datos: no se puede guardar."
                }
              >
                <Save className="h-4 w-4" />
                Guardar
              </Button>
            )}

            <ExportMenu
              artifact={artifact}
              rows={rows}
              ranAt={ranAt}
              monitorNames={monitorNames}
              chartContainerId={CONTAINER_ID}
            />
          </div>
        </div>

        {refreshDisabledReason && (
          <p className="text-xs text-warning-fg">{refreshDisabledReason}</p>
        )}
        {staleReason && <p className="text-xs text-warning-fg">{staleReason}</p>}
      </CardHeader>

      <CardContent className="flex-1 overflow-auto" id={CONTAINER_ID}>
        {/* Un artefacto recién creado con `sources` no trae ni caché ni
            `chartSpec.data`: sin esto, la primera vista mostraría "no hay
            resultados" durante el instante en que la corrida inicial
            todavía está en vuelo. */}
        {running && rows.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">Cargando resultados…</p>
        ) : (
          <>
            {artifact.type === "chart" && artifact.chartSpec && (
              <ChartArtifact spec={artifact.chartSpec} rows={rows} series={series} label={label} />
            )}
            {artifact.type === "table" && <TableArtifact data={rows} label={label} />}
            {artifact.type === "custom" && artifact.code && (
              <CustomArtifact code={artifact.code} />
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function ChartArtifact({
  spec,
  rows,
  series,
  label,
}: {
  spec: NonNullable<ChatArtifact["chartSpec"]>;
  /** Filas a graficar: SIEMPRE el estado `rows` del canvas, nunca `spec.data`. */
  rows: Record<string, unknown>[];
  /** Claves a graficar: SIEMPRE `run.series` (vía el estado `series` del
   *  canvas), nunca `spec.yKeys` — con varias fuentes el backend pivotea y
   *  renombra las columnas, y `yKeys` deja de apuntar a nada. */
  series: string[];
  label: (key: string) => string;
}) {
  if (spec.chartType === "pie") {
    const dataKey = series[0] ?? "value";
    return (
      <ResponsiveContainer width="100%" height={320}>
        <PieChart>
          <Pie
            data={rows}
            dataKey={dataKey}
            nameKey={spec.xKey ?? "name"}
            outerRadius={110}
            label
          >
            {rows.map((_, i) => (
              <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
            ))}
          </Pie>
          <RechartsTooltip formatter={(value: number) => [value, label(dataKey)]} />
        </PieChart>
      </ResponsiveContainer>
    );
  }

  if (spec.chartType === "line") {
    return (
      <ResponsiveContainer width="100%" height={320}>
        <LineChart data={rows}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey={spec.xKey} />
          <YAxis />
          <RechartsTooltip formatter={(value: number, name: string) => [value, label(name)]} />
          {series.map((key, i) => (
            <Line
              key={key}
              type="monotone"
              dataKey={key}
              name={label(key)}
              stroke={CHART_COLORS[i % CHART_COLORS.length]}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={320}>
      <BarChart data={rows}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey={spec.xKey} />
        <YAxis />
        <RechartsTooltip formatter={(value: number, name: string) => [value, label(name)]} />
        {series.map((key, i) => (
          <Bar
            key={key}
            dataKey={key}
            name={label(key)}
            fill={CHART_COLORS[i % CHART_COLORS.length]}
          />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}

function TableArtifact({
  data,
  label,
}: {
  data: Record<string, unknown>[];
  label: (key: string) => string;
}) {
  if (data.length === 0) {
    return (
      <p className="p-4 text-sm text-muted-foreground">
        La consulta no devolvió resultados.
      </p>
    );
  }
  const columns = unionColumns(data);

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            {columns.map((col) => (
              <th key={col} className="p-2 text-left font-medium">
                {label(col)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} className="border-b last:border-0">
              {columns.map((col) => {
                const kind = classifyValue(row[col]);
                return (
                  <td
                    key={col}
                    className={cn(
                      "p-2",
                      // Montos y timestamps en mono (manual de marca); los
                      // montos además a la derecha para comparar cifras de
                      // un vistazo por alineación de dígitos. Identificadores
                      // y hashes quedarían igual de bien en mono, pero
                      // distinguirlos de texto común requiere el tipo de
                      // campo del esquema del monitor, que esta tabla no
                      // recibe (llegan filas crudas): por ahora se quedan en
                      // la tipografía de lectura hasta que el trabajo de
                      // artefactos declare sus propias columnas.
                      kind === "number" && "text-right font-mono tabular-nums",
                      kind === "date" && "font-mono",
                      kind === "empty" && "text-muted-foreground",
                    )}
                  >
                    {formatValue(row[col])}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// CustomArtifact renderiza código generado por el LLM en un iframe
// sandboxeado — sandbox="allow-scripts" SIN allow-same-origin, SIN
// allow-top-navigation, SIN allow-popups. El iframe no tiene forma de leer
// cookies/localStorage/token de sesión ni de llamar a la API de la app: el
// único dato que ve es el que ya viene embebido en artifact.code (ver
// Global Constraints del plan).
function CustomArtifact({ code }: { code: string }) {
  return (
    <iframe
      srcDoc={code}
      sandbox="allow-scripts"
      className="w-full h-full min-h-[320px] border-0"
      title="Artefacto generado"
    />
  );
}
