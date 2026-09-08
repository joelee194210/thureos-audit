"use client";

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
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { ChatArtifact } from "@/lib/api/chat";

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

export function ArtifactCanvas({ artifact }: { artifact: ChatArtifact }) {
  return (
    <Card className="h-[70vh] flex flex-col">
      <CardHeader className="pb-2">
        <CardTitle className="text-base">{artifact.title}</CardTitle>
      </CardHeader>
      <CardContent className="flex-1 overflow-auto" id="artifact-canvas-content">
        {artifact.type === "chart" && artifact.chartSpec && (
          <ChartArtifact spec={artifact.chartSpec} />
        )}
        {artifact.type === "table" && artifact.chartSpec && (
          <TableArtifact data={artifact.chartSpec.data} />
        )}
        {artifact.type === "custom" && artifact.code && (
          <CustomArtifact code={artifact.code} />
        )}
      </CardContent>
    </Card>
  );
}

function ChartArtifact({
  spec,
}: {
  spec: NonNullable<ChatArtifact["chartSpec"]>;
}) {
  const yKeys = spec.yKeys ?? [];

  if (spec.chartType === "pie") {
    const dataKey = yKeys[0] ?? "value";
    return (
      <ResponsiveContainer width="100%" height={320}>
        <PieChart>
          <Pie
            data={spec.data}
            dataKey={dataKey}
            nameKey={spec.xKey ?? "name"}
            outerRadius={110}
            label
          >
            {spec.data.map((_, i) => (
              <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
            ))}
          </Pie>
          <RechartsTooltip />
        </PieChart>
      </ResponsiveContainer>
    );
  }

  if (spec.chartType === "line") {
    return (
      <ResponsiveContainer width="100%" height={320}>
        <LineChart data={spec.data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey={spec.xKey} />
          <YAxis />
          <RechartsTooltip />
          {yKeys.map((key, i) => (
            <Line
              key={key}
              type="monotone"
              dataKey={key}
              stroke={CHART_COLORS[i % CHART_COLORS.length]}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={320}>
      <BarChart data={spec.data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis dataKey={spec.xKey} />
        <YAxis />
        <RechartsTooltip />
        {yKeys.map((key, i) => (
          <Bar key={key} dataKey={key} fill={CHART_COLORS[i % CHART_COLORS.length]} />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}

function TableArtifact({ data }: { data: Record<string, unknown>[] }) {
  if (data.length === 0) return null;
  const columns = Object.keys(data[0]);

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b">
            {columns.map((col) => (
              <th key={col} className="text-left p-2 font-medium">
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, i) => (
            <tr key={i} className="border-b last:border-0">
              {columns.map((col) => (
                <td key={col} className="p-2">
                  {String(row[col] ?? "")}
                </td>
              ))}
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
