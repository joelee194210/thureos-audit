/**
 * Export de artefactos del chatbot: PNG para gráficos (serializa el SVG
 * que ya renderiza Recharts, sin librería nueva) y CSV para tablas.
 */

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

/** Reemplaza fill/stroke="var(--chart-N)" por su valor real resuelto —
 * el SVG clonado se serializa a un documento standalone sin acceso a las
 * custom properties del documento padre. */
function resolveChartColors(root: SVGElement) {
  const style = getComputedStyle(document.documentElement);

  function resolveElement(el: SVGElement) {
    for (const attr of ["fill", "stroke"] as const) {
      const value = el.getAttribute(attr);
      const match = value?.match(/^var\((--chart-\d+)\)$/);
      if (match) {
        const resolved = style.getPropertyValue(match[1]).trim();
        if (resolved) el.setAttribute(attr, resolved);
      }
    }
  }

  resolveElement(root);
  root.querySelectorAll<SVGElement>("[fill], [stroke]").forEach(resolveElement);
}

/** Serializa el primer <svg> dentro de containerId y lo baja como PNG. */
export function exportChartAsPNG(containerId: string, filename: string) {
  const container = document.getElementById(containerId);
  const svg = container?.querySelector("svg");
  if (!svg) return;

  const { width, height } = svg.getBoundingClientRect();
  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.setAttribute("width", String(width));
  clone.setAttribute("height", String(height));
  resolveChartColors(clone);

  const svgData = new XMLSerializer().serializeToString(clone);
  const svgBlob = new Blob([svgData], { type: "image/svg+xml;charset=utf-8" });
  const svgUrl = URL.createObjectURL(svgBlob);

  const img = new Image();
  img.onload = () => {
    const canvas = document.createElement("canvas");
    canvas.width = width * 2; // @2x para exportar nítido
    canvas.height = height * 2;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;
    ctx.scale(2, 2);
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, width, height);
    ctx.drawImage(img, 0, 0, width, height);
    URL.revokeObjectURL(svgUrl);

    canvas.toBlob((blob) => {
      if (blob) downloadBlob(blob, filename);
    }, "image/png");
  };
  img.src = svgUrl;
}

/** Arma un CSV simple (con comillas escapadas) y lo baja. */
export function exportTableAsCSV(
  data: Record<string, unknown>[],
  filename: string,
) {
  if (data.length === 0) return;
  const columns = Object.keys(data[0]);

  function escapeCell(value: unknown): string {
    const s = value === null || value === undefined ? "" : String(value);
    if (s.includes(",") || s.includes('"') || s.includes("\n")) {
      return `"${s.replace(/"/g, '""')}"`;
    }
    return s;
  }

  const lines = [
    columns.join(","),
    ...data.map((row) => columns.map((col) => escapeCell(row[col])).join(",")),
  ];
  const blob = new Blob([lines.join("\n")], {
    type: "text/csv;charset=utf-8",
  });
  downloadBlob(blob, filename);
}
