/**
 * Documento HTML autónomo e informe PDF de un artefacto del chat.
 *
 * Ambos formatos comparten la misma fuente de datos (`rows`, ya resueltos
 * por `artifactsApi.run` o por `cachedData` en modo degradado) y el mismo
 * formateo de celda (`formatValue`/`classifyValue`), para que el mismo
 * número no lea distinto en la tabla del chat, el PDF y este HTML.
 *
 * El HTML sale del navegador hacia un archivo o una pestaña nueva: no tiene
 * acceso a las custom properties de la app, así que los colores de marca se
 * resuelven en tiempo de generación con `readReportColors` (igual que el
 * PDF) y se embeben como `rgb()` literales en el <style>. Los únicos valores
 * que entran a ese <style> son esos tripletes numéricos — nunca texto de
 * datos ni el título, que solo aparecen escapados en el <body>.
 */
import jsPDF from "jspdf";
import autoTable from "jspdf-autotable";
import type { ChatArtifact } from "@/lib/api/chat";
import { classifyValue, formatValue, unionColumns } from "@/lib/format-value";
import { chartToPNGDataURL } from "@/lib/chat-export";
import { readReportColors, type RGB } from "@/lib/pdf-tokens";
import { formatDate } from "@/lib/utils";

/** Más columnas que esto son ilegibles en A4 vertical (igual que el informe de banderas rojas). */
const MAX_COLUMNS = 8;
const MARGIN = 14;
const HEADER_H = 24;

type AutoTableDoc = jsPDF & { lastAutoTable?: { finalY: number } };

/**
 * Escapa los cinco caracteres que importan en HTML. Es la única puerta por
 * la que un valor de dato o el título (ambos vienen de fuentes no
 * confiables: archivos subidos por usuarios y un título escrito por el LLM)
 * puede llegar al documento — nunca se inserta marcado ajeno sin pasar por
 * acá primero. El `&` va primero: si fuera después, escaparía las entidades
 * que este mismo reemplazo acaba de crear.
 */
export function escapeHTML(value: unknown): string {
  const s = value === null || value === undefined ? "" : String(value);
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

/**
 * `ranAt` es opcional en `ChatArtifact` (un artefacto instantánea, sin
 * `sources`, nunca corrió) y `formatDate` no perdona una fecha ausente o
 * inválida: `new Date("")` es "Invalid Date", y `Intl.DateTimeFormat.
 * format()` sobre eso lanza `RangeError: Invalid time value` en vez de
 * devolver texto. Mismo patrón de guarda que `classifyValue` en
 * format-value.ts: se valida ANTES de formatear, nunca se deja que el
 * formateador reciba lo que no sabe manejar.
 */
export function formatRanAt(ranAt: string | undefined): string {
  if (!ranAt) return "sin fecha de corrida";
  const parsed = new Date(ranAt);
  if (Number.isNaN(parsed.getTime())) return "sin fecha de corrida";
  return formatDate(parsed);
}

function slugify(text: string): string {
  return text
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "")
    .slice(0, 60);
}

/** Columnas a mostrar y en qué orden: la proyección del artefacto, o todas si no la declaró. */
function resolveColumns(
  artifact: ChatArtifact,
  rows: Record<string, unknown>[],
): string[] {
  const declared = artifact.chartSpec?.columns;
  return declared && declared.length > 0 ? declared : unionColumns(rows);
}

function resolveLabel(artifact: ChatArtifact, column: string): string {
  return artifact.chartSpec?.labels?.[column] ?? column;
}

function rgbCss([r, g, b]: RGB): string {
  return `rgb(${r}, ${g}, ${b})`;
}

/** Resuelve la paleta de marca (tema claro: es la del papel/pantalla de un reporte exportable). */
function reportColors() {
  return readReportColors({
    ink: "--fg-default",
    muted: "--fg-muted",
    line: "--border-default",
    surface: "--bg-surface",
    surface2: "--bg-surface-2",
    canvas: "--bg-canvas",
    accent: "--accent-fg",
  });
}

/**
 * Arma el documento HTML completo (estilos embebidos, sin recursos
 * externos). Es una función pura: no toca `document` salvo indirectamente a
 * través de `readReportColors`, que ya cae a un gris de respaldo fuera del
 * navegador — por eso es segura de probar en un entorno sin DOM.
 *
 * `chartImage` es el data URI ya resuelto (ver `chartToPNGDataURL`): esta
 * función no lo genera, porque generarlo requiere el DOM real y romperla
 * la pureza que la hace testeable.
 */
export function buildArtifactHTML(
  artifact: ChatArtifact,
  rows: Record<string, unknown>[],
  ranAt: string | undefined,
  monitorNames: string[],
  chartImage?: string | null,
): string {
  // Un artefacto `custom` ES el código HTML que generó el LLM (el mismo que
  // ya se renderiza sandboxeado en un <iframe> en artifact-canvas.tsx) — no
  // una tabla que envolver en la plantilla de marca. Insertarlo DENTRO de
  // esa plantilla, concatenado junto a texto escapado, sería precisamente
  // el vector que la constraint de escapado prohíbe: marcado no confiable
  // mezclado con el resto del documento. Servirlo tal cual, como su propio
  // archivo/pestaña, no es más riesgoso que el iframe que la app ya
  // renderiza para ese mismo contenido — así que no pasa por acá abajo.
  if (artifact.type === "custom") {
    return artifact.code ?? "<!doctype html><html><body></body></html>";
  }

  const c = reportColors();
  const generatedAt = new Date();
  const title = escapeHTML(artifact.title || "Artefacto");
  const columns = resolveColumns(artifact, rows);

  const monitorChips = monitorNames.length
    ? `<div class="chips">${monitorNames
        .map((name) => `<span class="chip">${escapeHTML(name)}</span>`)
        .join("")}</div>`
    : "";

  // Solo se embebe si es un data URI de imagen: una cadena vacía o cualquier
  // otra cosa (ej. una promesa rechazada resuelta a null) se descarta en
  // silencio en vez de intentar volverla marcado.
  const chartBlock =
    artifact.type === "chart" && chartImage?.startsWith("data:image/")
      ? `<div class="chart"><img src="${escapeHTML(chartImage)}" alt="${title}" /></div>`
      : "";

  let tableBlock: string;
  if (rows.length === 0) {
    tableBlock = `<p class="empty">La consulta no devolvió resultados.</p>`;
  } else {
    const head = columns
      .map((col) => `<th>${escapeHTML(resolveLabel(artifact, col))}</th>`)
      .join("");
    const body = rows
      .map((row) => {
        const cells = columns
          .map((col) => {
            const kind = classifyValue(row[col]);
            const cls =
              kind === "number"
                ? ' class="num"'
                : kind === "date"
                  ? ' class="mono"'
                  : kind === "empty"
                    ? ' class="muted"'
                    : "";
            return `<td${cls}>${escapeHTML(formatValue(row[col]))}</td>`;
          })
          .join("");
        return `<tr>${cells}</tr>`;
      })
      .join("");
    tableBlock = `<table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table>`;
  }

  return `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>${title}</title>
<style>
  :root {
    --ink: ${rgbCss(c.ink)};
    --muted: ${rgbCss(c.muted)};
    --line: ${rgbCss(c.line)};
    --surface: ${rgbCss(c.surface)};
    --surface-2: ${rgbCss(c.surface2)};
    --canvas: ${rgbCss(c.canvas)};
    --accent: ${rgbCss(c.accent)};
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    padding: 32px;
    background: var(--canvas);
    color: var(--ink);
    font-family: "Inter", ui-sans-serif, system-ui, -apple-system, sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }
  header.report-header {
    border-bottom: 1px solid var(--line);
    padding-bottom: 16px;
    margin-bottom: 24px;
  }
  .brand {
    color: var(--accent);
    font-weight: 700;
    font-size: 13px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  h1 {
    font-size: 22px;
    font-weight: 700;
    margin: 6px 0 10px;
  }
  .meta {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    color: var(--muted);
    font-family: "JetBrains Mono", ui-monospace, "SFMono-Regular", Menlo, monospace;
    font-size: 12px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: 12px;
  }
  .chip {
    background: var(--surface-2);
    color: var(--ink);
    border-radius: 999px;
    padding: 3px 10px;
    font-size: 12px;
  }
  .chart {
    margin-bottom: 24px;
    padding: 12px;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 8px;
  }
  .chart img { max-width: 100%; height: auto; display: block; margin: 0 auto; }
  table {
    width: 100%;
    border-collapse: collapse;
    background: var(--surface);
    border: 1px solid var(--line);
    border-radius: 8px;
    overflow: hidden;
  }
  th, td {
    padding: 8px 10px;
    border-bottom: 1px solid var(--line);
    text-align: left;
    font-size: 13px;
  }
  th {
    background: var(--surface-2);
    font-weight: 600;
    color: var(--muted);
    text-transform: uppercase;
    font-size: 11px;
    letter-spacing: 0.03em;
  }
  tr:last-child td { border-bottom: none; }
  td.num {
    text-align: right;
    font-family: "JetBrains Mono", ui-monospace, "SFMono-Regular", Menlo, monospace;
    font-variant-numeric: tabular-nums;
  }
  td.mono {
    font-family: "JetBrains Mono", ui-monospace, "SFMono-Regular", Menlo, monospace;
  }
  td.muted { color: var(--muted); }
  p.empty { color: var(--muted); font-style: italic; }
  footer {
    margin-top: 24px;
    padding-top: 12px;
    border-top: 1px solid var(--line);
    color: var(--muted);
    font-size: 11px;
  }
</style>
</head>
<body>
<header class="report-header">
  <div class="brand">Thureos Compliance</div>
  <h1>${title}</h1>
  <div class="meta">
    <span>Datos al: ${escapeHTML(formatRanAt(ranAt))}</span>
    <span>Generado: ${escapeHTML(formatDate(generatedAt))}</span>
  </div>
  ${monitorChips}
</header>
<main>
  ${chartBlock}
  ${tableBlock}
</main>
<footer>Thureos Compliance — documento generado automáticamente, sin necesidad de conexión para su lectura.</footer>
</body>
</html>
`;
}

/** Resuelve la imagen del gráfico si corresponde; null para tablas/custom o si no hay contenedor. */
async function resolveChartImage(
  artifact: ChatArtifact,
  chartContainerId?: string,
): Promise<string | null> {
  if (artifact.type !== "chart" || !chartContainerId) return null;
  return chartToPNGDataURL(chartContainerId);
}

/** Arma el HTML y lo descarga como archivo `.html`. */
export async function downloadArtifactHTML(
  artifact: ChatArtifact,
  rows: Record<string, unknown>[],
  ranAt: string | undefined,
  monitorNames: string[],
  chartContainerId?: string,
): Promise<void> {
  const chartImage = await resolveChartImage(artifact, chartContainerId);
  const html = buildArtifactHTML(artifact, rows, ranAt, monitorNames, chartImage);
  const blob = new Blob([html], { type: "text/html;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  // Con fecha, como el PDF: dos descargas del mismo artefacto en días
  // distintos no colisionan en "foo.html" / "foo (1).html", que no dice
  // cuál es la más reciente (la razón de imprimir la fecha de corrida).
  const stamp = new Date().toISOString().slice(0, 10);
  a.download = `${slugify(artifact.title) || "artefacto"}-${stamp}.html`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

/** Arma el HTML y lo abre en una pestaña nueva. */
export async function openArtifactHTML(
  artifact: ChatArtifact,
  rows: Record<string, unknown>[],
  ranAt: string | undefined,
  monitorNames: string[],
  chartContainerId?: string,
): Promise<void> {
  // La pestaña se abre ANTES del await de la imagen: abrirla después de una
  // espera asíncrona pierde el gesto de usuario y la mayoría de navegadores
  // la bloquea como pop-up.
  const win = window.open("", "_blank");
  const chartImage = await resolveChartImage(artifact, chartContainerId);
  if (!win) return; // Bloqueada por el navegador: nada más que hacer.
  const html = buildArtifactHTML(artifact, rows, ranAt, monitorNames, chartImage);
  // Se navega la pestaña a un Blob URL en vez de document.write: evita el
  // patrón de inyección de document.write sobre un documento vivo, aunque
  // acá el HTML ya salga escapado. El URL no se revoca: la pestaña lo sigue
  // necesitando después de esta función y se libera solo al cerrarla.
  const blob = new Blob([html], { type: "text/html;charset=utf-8" });
  win.location.href = URL.createObjectURL(blob);
}

/**
 * Informe PDF de un artefacto: sigue el patrón de `red-flag-report.ts`
 * (`readReportColors`, cabecera de marca, `autoTable`).
 *
 * `custom` queda afuera a propósito: es código HTML, no datos tabulares, y
 * no hay una tabla razonable que autoTable pueda dibujar a partir de él
 * (mismo criterio que excluye a `custom` del export a Excel en el backend).
 * Se lanza en vez de generar un PDF vacío o sin sentido, para que quien
 * llame lo note y no ofrezca el botón para este caso.
 */
export async function exportArtifactPDF(
  artifact: ChatArtifact,
  rows: Record<string, unknown>[],
  ranAt: string | undefined,
  monitorNames: string[],
  chartContainerId?: string,
): Promise<void> {
  if (artifact.type === "custom") {
    throw new Error(
      "Los artefactos personalizados no tienen informe en PDF: son código, no datos tabulares.",
    );
  }

  const chartImage = await resolveChartImage(artifact, chartContainerId);
  const c = reportColors();

  const doc = new jsPDF({ unit: "mm", format: "a4" }) as AutoTableDoc;
  const pageW = doc.internal.pageSize.getWidth();
  const pageH = doc.internal.pageSize.getHeight();
  const contentW = pageW - MARGIN * 2;
  const generatedAt = new Date();

  let y = HEADER_H + 10;
  const setColor = (rgb: RGB) => doc.setTextColor(rgb[0], rgb[1], rgb[2]);

  // ---- Título ---------------------------------------------------------
  doc.setFont("helvetica", "bold");
  doc.setFontSize(15);
  setColor(c.ink);
  const titleLines = doc.splitTextToSize(
    artifact.title || "Artefacto",
    contentW,
  ) as string[];
  doc.text(titleLines, MARGIN, y);
  y += titleLines.length * 6.5 + 4;

  // ---- Fechas: la de los datos y la de la generación, por separado -----
  // (constraint del plan: todo export imprime su fecha de corrida; en modo
  // degradado esta es la que dice si la data mostrada quedó vieja).
  doc.setFont("helvetica", "normal");
  doc.setFontSize(9);
  setColor(c.muted);
  doc.text(`Datos al: ${formatRanAt(ranAt)}`, MARGIN, y);
  doc.text(`Generado: ${formatDate(generatedAt)}`, MARGIN + contentW / 2, y);
  y += 7;

  // ---- Chips de monitores consultados -----------------------------------
  if (monitorNames.length > 0) {
    doc.setFontSize(8.5);
    const chip = (text: string, x: number, w: number): number => {
      doc.setFillColor(c.surface2[0], c.surface2[1], c.surface2[2]);
      doc.roundedRect(x, y - 3.6, w, 5.8, 1.2, 1.2, "F");
      setColor(c.ink);
      doc.text(text, x + 3, y);
      return x + w + 3;
    };
    let chipX = MARGIN;
    for (const name of monitorNames) {
      const w = doc.getTextWidth(name) + 6;
      // Se decide con el ANCHO del chip que sigue, no con la posición de
      // arranque: un nombre de monitor largo empezaba dentro del umbral y
      // se dibujaba de todos modos fuera del margen derecho.
      if (chipX !== MARGIN && chipX + w > MARGIN + contentW) {
        y += 8;
        chipX = MARGIN;
      }
      chipX = chip(name, chipX, w);
    }
    y += 8;
  }

  // ---- Gráfico (si aplica) ----------------------------------------------
  if (chartImage) {
    try {
      const props = doc.getImageProperties(chartImage);
      const naturalH = (props.height / props.width) * contentW;
      // Si no cabe ni una fracción razonable en lo que queda de la página
      // actual, se pasa a una nueva ANTES de escalar: mejor un gráfico
      // grande en su propia página que uno empequeñecido al fondo de la
      // anterior. Escalar primero contra el espacio viejo y decidir el
      // salto de página después podía forzarlo de todos modos (el margen
      // inferior no entraba en la cuenta), dejando la imagen encogida sin
      // necesidad.
      if (y + Math.min(naturalH, 60) > pageH - 20) {
        doc.addPage();
        y = HEADER_H + 10;
      }
      let imgW = contentW;
      let imgH = naturalH;
      const maxH = pageH - y - 20;
      if (imgH > maxH) {
        const scale = maxH / imgH;
        imgH *= scale;
        imgW *= scale;
      }
      doc.addImage(
        chartImage,
        "PNG",
        MARGIN + (contentW - imgW) / 2,
        y,
        imgW,
        imgH,
      );
      y += imgH + 8;
    } catch {
      // Una imagen corrupta o de formato inesperado no debe tumbar el
      // informe entero: se omite y el PDF sigue con la tabla.
    }
  }

  // ---- Tabla de datos -----------------------------------------------------
  if (rows.length === 0) {
    doc.setFont("helvetica", "italic");
    doc.setFontSize(9.5);
    setColor(c.muted);
    doc.text("La consulta no devolvió resultados.", MARGIN, y);
  } else {
    const allColumns = resolveColumns(artifact, rows);
    const columns = allColumns.slice(0, MAX_COLUMNS);
    const labels = columns.map((col) => resolveLabel(artifact, col));

    autoTable(doc, {
      startY: y,
      theme: "striped",
      margin: { left: MARGIN, right: MARGIN, top: HEADER_H + 6 },
      styles: {
        fontSize: 7.5,
        cellPadding: 1.8,
        overflow: "linebreak",
        textColor: c.ink,
        lineColor: c.line,
      },
      headStyles: { fillColor: c.surface2, textColor: c.ink, fontStyle: "bold" },
      alternateRowStyles: { fillColor: c.surface },
      head: [labels],
      body: rows.map((row) => columns.map((col) => formatValue(row[col]))),
    });
    y = (doc.lastAutoTable?.finalY ?? y) + 4;

    if (allColumns.length > columns.length) {
      doc.setFont("helvetica", "italic");
      doc.setFontSize(8.5);
      setColor(c.muted);
      const note = `Se muestran ${columns.length} de ${allColumns.length} columnas; las restantes se omiten por espacio.`;
      doc.text(doc.splitTextToSize(note, contentW) as string[], MARGIN, y);
    }
  }

  // ---- Encabezado y pie en todas las páginas ------------------------------
  const pages = doc.getNumberOfPages();
  for (let i = 1; i <= pages; i++) {
    doc.setPage(i);

    doc.setFillColor(c.surface[0], c.surface[1], c.surface[2]);
    doc.rect(0, 0, pageW, HEADER_H, "F");
    doc.setFont("helvetica", "bold");
    doc.setFontSize(11);
    setColor(c.accent);
    doc.text("Thureos Compliance", MARGIN, 11);
    doc.setFont("helvetica", "normal");
    doc.setFontSize(9);
    setColor(c.muted);
    doc.text("Informe de artefacto", MARGIN, 17);
    doc.text(formatDate(generatedAt), pageW - MARGIN, 17, { align: "right" });

    doc.setDrawColor(c.line[0], c.line[1], c.line[2]);
    doc.setLineWidth(0.2);
    doc.line(MARGIN, pageH - 12, pageW - MARGIN, pageH - 12);
    doc.setFontSize(7.5);
    setColor(c.muted);
    doc.text(`Página ${i} de ${pages}`, pageW - MARGIN, pageH - 8, {
      align: "right",
    });
  }

  const stamp = generatedAt.toISOString().slice(0, 10);
  doc.save(`informe-artefacto-${slugify(artifact.title) || "artefacto"}-${stamp}.pdf`);
}
