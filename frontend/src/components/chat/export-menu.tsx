"use client";

import { useState } from "react";
import { Download, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { ChatArtifact } from "@/lib/api/chat";
import { artifactsApi } from "@/lib/api/artifacts";
import { ApiError } from "@/lib/api/client";
import {
  availableFormats,
  unavailableFormatsNotice,
  type ExportFormat,
} from "@/lib/artifact-formats";
import { exportChartAsPNG, exportTableAsCSV } from "@/lib/chat-export";
import {
  downloadArtifactHTML,
  exportArtifactPDF,
  openArtifactHTML,
} from "@/lib/artifact-report";
import { useToast } from "@/lib/use-toast";

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

interface ExportMenuProps {
  artifact: ChatArtifact;
  /** Filas actualmente pintadas en el canvas (caché o corrida fresca). */
  rows: Record<string, unknown>[];
  ranAt: string | undefined;
  monitorNames: string[];
  /** id del contenedor del gráfico en el DOM, para PNG/PDF/HTML embebidos. */
  chartContainerId?: string;
  /**
   * Aviso del canvas de que `rows` es una caché rancia (corrida fallida o
   * 409). El PDF y el HTML ya estampan `ranAt` adentro del documento; el
   * CSV y el PNG no llevan ninguna fecha, así que sin este aviso en el
   * propio menú —ANTES de descargar— alguien se lleva un archivo
   * indistinguible de uno fresco.
   */
  staleReason?: string | null;
}

/**
 * Menú de exportación por tipo de artefacto. La tabla de formatos vive en
 * `lib/artifact-formats.ts` (`availableFormats`) — este componente solo la
 * consulta y ejecuta cada acción.
 */
export function ExportMenu({
  artifact,
  rows,
  ranAt,
  monitorNames,
  chartContainerId,
  staleReason,
}: ExportMenuProps) {
  const { toastError } = useToast();
  const [busy, setBusy] = useState<ExportFormat | null>(null);

  const formats = availableFormats(artifact);
  const notice = unavailableFormatsNotice(artifact);

  async function downloadXlsx() {
    if (!artifact.id) {
      toastError(
        "Este artefacto todavía no quedó vinculado a datos: no se puede exportar a Excel.",
      );
      return;
    }
    try {
      const blob = await artifactsApi.exportXlsx(artifact.id);
      if (!blob) {
        toastError("Artefacto no encontrado.");
        return;
      }
      downloadBlob(blob, `${artifact.title}.xlsx`);
    } catch (err) {
      // 409 = modo degradado del spec: el monitor de origen ya no existe.
      // El backend deliberadamente no exporta datos viejos sin aviso acá
      // (una descarga no tiene dónde mostrar una nota), así que el error
      // tiene que decir exactamente eso, no un "no se pudo exportar" genérico.
      if (err instanceof ApiError && err.status === 409) {
        toastError(
          "No se pudo exportar a Excel: el monitor de origen de este artefacto ya no existe.",
        );
      } else {
        toastError("No se pudo exportar a Excel.");
      }
    }
  }

  async function handle(format: ExportFormat) {
    setBusy(format);
    try {
      switch (format) {
        case "png":
          if (chartContainerId) {
            await exportChartAsPNG(chartContainerId, `${artifact.title}.png`);
          }
          break;
        case "csv":
          exportTableAsCSV(rows, `${artifact.title}.csv`);
          break;
        case "xlsx":
          await downloadXlsx();
          break;
        case "pdf":
          // No se ofrece para `custom` (availableFormats ya lo excluye);
          // exportArtifactPDF lanza para ese tipo, así que este camino
          // nunca lo alcanza con ese artefacto.
          await exportArtifactPDF(artifact, rows, ranAt, monitorNames, chartContainerId);
          break;
        case "html":
          await downloadArtifactHTML(artifact, rows, ranAt, monitorNames, chartContainerId);
          break;
        case "print":
          await openArtifactHTML(artifact, rows, ranAt, monitorNames, chartContainerId);
          break;
      }
    } catch {
      toastError("No se pudo exportar el artefacto.");
    } finally {
      setBusy(null);
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm" disabled={busy !== null}>
          {busy ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Download className="h-4 w-4" />
          )}
          Exportar
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Exportar como</DropdownMenuLabel>
        {formats.map((f) => (
          <DropdownMenuItem key={f.format} onClick={() => handle(f.format)}>
            {f.label}
          </DropdownMenuItem>
        ))}
        {staleReason && (
          <>
            <DropdownMenuSeparator />
            <p className="px-2 py-1.5 text-xs text-warning-fg">
              Datos desactualizados: {staleReason}
            </p>
          </>
        )}
        {notice && (
          <>
            <DropdownMenuSeparator />
            <p className="px-2 py-1.5 text-xs text-muted-foreground">{notice}</p>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
