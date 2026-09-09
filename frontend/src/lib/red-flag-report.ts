/**
 * Informe PDF de una bandera roja: se arma en el navegador con los mismos
 * datos que ya expone la API (ficha, registros afectados y bitácora de
 * auditoría), sin endpoint nuevo en el backend.
 */
import jsPDF from "jspdf";
import autoTable from "jspdf-autotable";
import { esAgrupada, type RedFlag, type RedFlagLog } from "@/lib/types";
import { redFlagsApi } from "@/lib/api/red-flags";
import {
  RED_FLAG_STATUS_LABELS,
  ACTION_CATEGORY_LABELS,
  SEVERITY_LABELS,
} from "@/lib/red-flag-labels";
import { readReportColors, type RGB } from "@/lib/pdf-tokens";
import { formatDate } from "@/lib/utils";

/** Tope del backend en `GET /red-flags/:id/records`; pedir más lo baja a 50. */
const MAX_RECORDS = 500;
/** Más columnas que esto son ilegibles en A4 vertical. */
const MAX_COLUMNS = 8;
const MARGIN = 14;
const HEADER_H = 24;

type AutoTableDoc = jsPDF & { lastAutoTable?: { finalY: number } };

function formatValue(val: unknown): string {
  if (val === null || val === undefined || val === "") return "—";
  if (typeof val === "number")
    return new Intl.NumberFormat("es-CO").format(val);
  if (typeof val === "boolean") return val ? "Sí" : "No";
  if (typeof val === "object") return JSON.stringify(val);
  const s = String(val);
  // Fechas ISO que llegan como texto desde colecciones dinámicas.
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}/.test(s)) {
    const d = new Date(s);
    if (!isNaN(d.getTime())) return formatDate(s);
  }
  return s;
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

export async function generateRedFlagReport(redFlag: RedFlag): Promise<void> {
  // Ninguna de las dos cargas es obligatoria: si una falla, el informe sale
  // con el resto y lo dice, en vez de no salir.
  const [recordsResult, logs] = await Promise.all([
    redFlagsApi.records(redFlag.id, 1, MAX_RECORDS).catch(() => null),
    redFlagsApi.getLogs(redFlag.id).catch(() => null),
  ]);

  const embedded = (redFlag.matchedRecords ??
    (redFlag.matchedData ? [redFlag.matchedData] : [])) as Record<
    string,
    unknown
  >[];
  const records = recordsResult?.records?.length
    ? recordsResult.records
    : embedded;
  const totalRecords =
    recordsResult?.total ?? redFlag.matchCount ?? records.length;

  const c = readReportColors({
    ink: "--fg-default",
    muted: "--fg-muted",
    line: "--border-default",
    surface: "--bg-surface-2",
    accent: "--accent-fg",
    riskFg: `--risk-${redFlag.severity}-fg`,
    riskBg: `--risk-${redFlag.severity}-bg`,
  });

  const doc = new jsPDF({ unit: "mm", format: "a4" }) as AutoTableDoc;
  const pageW = doc.internal.pageSize.getWidth();
  const pageH = doc.internal.pageSize.getHeight();
  const contentW = pageW - MARGIN * 2;
  const generatedAt = new Date();

  let y = HEADER_H + 10;

  const ensureSpace = (needed: number) => {
    if (y + needed > pageH - 20) {
      doc.addPage();
      y = HEADER_H + 10;
    }
  };

  const setColor = (rgb: RGB) => doc.setTextColor(rgb[0], rgb[1], rgb[2]);

  // ---- Título y estado ----------------------------------------------------
  doc.setFont("helvetica", "bold");
  doc.setFontSize(15);
  setColor(c.ink);
  const titleLines = doc.splitTextToSize(
    redFlag.ruleName,
    contentW,
  ) as string[];
  doc.text(titleLines, MARGIN, y);
  y += titleLines.length * 6.5 + 2;

  // Chips de severidad y estado.
  doc.setFontSize(8.5);
  const chip = (text: string, fg: RGB, bg: RGB, x: number): number => {
    const w = doc.getTextWidth(text) + 6;
    doc.setFillColor(bg[0], bg[1], bg[2]);
    doc.roundedRect(x, y - 3.6, w, 5.8, 1.2, 1.2, "F");
    setColor(fg);
    doc.text(text, x + 3, y);
    return x + w + 3;
  };
  let chipX = MARGIN;
  chipX = chip(
    `Severidad: ${SEVERITY_LABELS[redFlag.severity]}`,
    c.riskFg,
    c.riskBg,
    chipX,
  );
  chip(
    `Estado: ${RED_FLAG_STATUS_LABELS[redFlag.status]}`,
    c.ink,
    c.surface,
    chipX,
  );
  y += 8;

  if (redFlag.message) {
    doc.setFont("helvetica", "normal");
    doc.setFontSize(9.5);
    setColor(c.muted);
    const msg = doc.splitTextToSize(redFlag.message, contentW) as string[];
    doc.text(msg, MARGIN, y);
    y += msg.length * 4.6 + 4;
  }

  // ---- Ficha --------------------------------------------------------------
  const isAggregate = esAgrupada(redFlag.redFlagType);
  const summary: [string, string][] = [
    ["Identificador", redFlag.id],
    ["Monitor", redFlag.monitorName],
    ["Regla", redFlag.ruleName],
    ["Tipo", isAggregate ? "Agregada" : "Por coincidencia"],
    ["Detectada", formatDate(redFlag.createdAt)],
    ["Última actualización", formatDate(redFlag.updatedAt)],
    ["Coincidencias", formatValue(redFlag.matchCount)],
  ];
  if (isAggregate) {
    summary.push([
      "Agregación",
      `${(redFlag.aggFunction ?? "").toUpperCase()}(${redFlag.aggField ?? "—"}) por ${redFlag.groupByField ?? "—"}`,
    ]);
    if (redFlag.groupByValue) summary.push(["Grupo", redFlag.groupByValue]);
    if (redFlag.aggValue !== undefined)
      summary.push(["Valor", formatValue(redFlag.aggValue)]);
    if (redFlag.threshold !== undefined)
      summary.push(["Umbral", formatValue(redFlag.threshold)]);
    if (redFlag.aggValue !== undefined && redFlag.threshold) {
      summary.push([
        "Sobre el umbral",
        `${((redFlag.aggValue / redFlag.threshold) * 100).toFixed(0)}%`,
      ]);
    }
  }

  y = section(
    doc,
    "Ficha de la bandera roja",
    y,
    c,
    MARGIN,
    contentW,
    ensureSpace,
  );
  autoTable(doc, {
    startY: y,
    theme: "plain",
    margin: { left: MARGIN, right: MARGIN, top: HEADER_H + 6 },
    styles: {
      fontSize: 9,
      cellPadding: { top: 1.6, bottom: 1.6, left: 0, right: 2 },
    },
    columnStyles: {
      0: { cellWidth: 45, textColor: c.muted },
      1: { textColor: c.ink, fontStyle: "bold" },
    },
    body: summary,
  });
  y = (doc.lastAutoTable?.finalY ?? y) + 8;

  // ---- Registros afectados ------------------------------------------------
  y = section(doc, "Registros afectados", y, c, MARGIN, contentW, ensureSpace);
  if (records.length === 0) {
    y = note(
      doc,
      recordsResult === null
        ? "No fue posible cargar los registros afectados."
        : "Esta bandera roja no tiene registros asociados.",
      y,
      c,
      MARGIN,
      contentW,
    );
  } else {
    const allColumns = Object.keys(records[0]).filter(
      (k) => !k.startsWith("_"),
    );
    const columns = allColumns.slice(0, MAX_COLUMNS);
    autoTable(doc, {
      startY: y,
      theme: "striped",
      margin: { left: MARGIN, right: MARGIN, top: HEADER_H + 6 },
      styles: {
        fontSize: 7,
        cellPadding: 1.5,
        overflow: "linebreak",
        textColor: c.ink,
        lineColor: c.line,
      },
      headStyles: { fillColor: c.surface, textColor: c.ink, fontStyle: "bold" },
      alternateRowStyles: { fillColor: [248, 250, 252] },
      head: [columns],
      body: records.map((rec) => columns.map((col) => formatValue(rec[col]))),
    });
    y = (doc.lastAutoTable?.finalY ?? y) + 4;

    const notes: string[] = [];
    if (records.length < totalRecords) {
      notes.push(
        `Se incluyen ${records.length} de ${totalRecords} registros (el informe se limita a los primeros ${MAX_RECORDS}).`,
      );
    }
    if (allColumns.length > columns.length) {
      notes.push(
        `Se muestran ${columns.length} de ${allColumns.length} columnas; las restantes se omiten por espacio.`,
      );
    }
    if (notes.length) y = note(doc, notes.join(" "), y, c, MARGIN, contentW);
    y += 4;
  }

  // ---- Bitácora -----------------------------------------------------------
  y = section(
    doc,
    "Bitácora de auditoría",
    y,
    c,
    MARGIN,
    contentW,
    ensureSpace,
  );
  if (logs === null) {
    note(
      doc,
      "No fue posible cargar la bitácora de auditoría.",
      y,
      c,
      MARGIN,
      contentW,
    );
  } else if (logs.length === 0) {
    note(
      doc,
      "Todavía no se han registrado acciones sobre esta bandera roja.",
      y,
      c,
      MARGIN,
      contentW,
    );
  } else {
    autoTable(doc, {
      startY: y,
      theme: "grid",
      margin: { left: MARGIN, right: MARGIN, top: HEADER_H + 6 },
      styles: {
        fontSize: 7.5,
        cellPadding: 1.8,
        overflow: "linebreak",
        textColor: c.ink,
        lineColor: c.line,
      },
      headStyles: { fillColor: c.surface, textColor: c.ink, fontStyle: "bold" },
      // Estado anterior y nuevo en columnas separadas: la fuente estándar de
      // jsPDF no tiene glifo para la flecha "→" y la imprimiría como basura.
      columnStyles: {
        0: { cellWidth: 24 },
        1: { cellWidth: 30 },
        2: { cellWidth: 22 },
        3: { cellWidth: 22 },
        4: { cellWidth: 24 },
      },
      head: [
        [
          "Fecha",
          "Usuario",
          "Estado anterior",
          "Estado nuevo",
          "Categoría",
          "Notas",
        ],
      ],
      body: logs.map((log: RedFlagLog) => [
        formatDate(log.createdAt),
        log.userName || log.userEmail || "—",
        RED_FLAG_STATUS_LABELS[log.previousStatus] ?? log.previousStatus,
        RED_FLAG_STATUS_LABELS[log.newStatus] ?? log.newStatus,
        ACTION_CATEGORY_LABELS[log.category] ?? log.category,
        log.notes || "—",
      ]),
    });
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
    doc.text("Informe de bandera roja", MARGIN, 17);
    doc.text(formatDate(generatedAt), pageW - MARGIN, 17, { align: "right" });

    doc.setDrawColor(c.line[0], c.line[1], c.line[2]);
    doc.setLineWidth(0.2);
    doc.line(MARGIN, pageH - 12, pageW - MARGIN, pageH - 12);
    doc.setFontSize(7.5);
    setColor(c.muted);
    doc.text(`Bandera roja ${redFlag.id}`, MARGIN, pageH - 8);
    doc.text(`Página ${i} de ${pages}`, pageW - MARGIN, pageH - 8, {
      align: "right",
    });
  }

  const stamp = generatedAt.toISOString().slice(0, 10);
  doc.save(`informe-bandera-roja-${slugify(redFlag.ruleName)}-${stamp}.pdf`);
}

/** Título de sección con su línea divisoria. Devuelve la y siguiente. */
function section(
  doc: jsPDF,
  title: string,
  y: number,
  c: Record<string, RGB>,
  margin: number,
  contentW: number,
  ensureSpace: (needed: number) => void,
): number {
  ensureSpace(24);
  doc.setFont("helvetica", "bold");
  doc.setFontSize(10.5);
  doc.setTextColor(c.ink[0], c.ink[1], c.ink[2]);
  doc.text(title, margin, y);
  doc.setDrawColor(c.line[0], c.line[1], c.line[2]);
  doc.setLineWidth(0.2);
  doc.line(margin, y + 1.8, margin + contentW, y + 1.8);
  return y + 7;
}

/** Nota aclaratoria en gris. Devuelve la y siguiente. */
function note(
  doc: jsPDF,
  text: string,
  y: number,
  c: Record<string, RGB>,
  margin: number,
  contentW: number,
): number {
  doc.setFont("helvetica", "italic");
  doc.setFontSize(8.5);
  doc.setTextColor(c.muted[0], c.muted[1], c.muted[2]);
  const lines = doc.splitTextToSize(text, contentW) as string[];
  doc.text(lines, margin, y);
  return y + lines.length * 4.2 + 2;
}
