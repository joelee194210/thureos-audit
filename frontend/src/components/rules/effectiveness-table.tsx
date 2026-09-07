"use client";

import { useEffect, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { useToast } from "@/lib/use-toast";
import { rulesApi, type EffectivenessStats } from "@/lib/api/rules";
import type { Rule } from "@/lib/types";

interface Row {
  ruleId: string;
  name: string;
  stats: EffectivenessStats;
}

type SortKey = "name" | "total" | "falsePositiveRate" | "avgCloseHours";

interface EffectivenessTableProps {
  rules: Rule[];
}

interface SortHeaderProps {
  label: string;
  sortKey: SortKey;
  activeKey: SortKey;
  desc: boolean;
  onSort: (key: SortKey) => void;
}

function SortHeader({ label, sortKey, activeKey, desc, onSort }: SortHeaderProps) {
  const Icon = activeKey !== sortKey ? ArrowUpDown : desc ? ArrowDown : ArrowUp;
  return (
    <button
      type="button"
      onClick={() => onSort(sortKey)}
      className="flex items-center gap-1 font-medium text-muted-foreground hover:text-foreground"
    >
      {label}
      <Icon className="h-3 w-3" />
    </button>
  );
}

// Vista dedicada, no un widget del dashboard configurable: los widgets
// agregan sobre datos de monitor, esto agrega sobre metadata de casos —
// son sistemas distintos con objetivos distintos.
export function EffectivenessTable({ rules }: EffectivenessTableProps) {
  const { toastError } = useToast();
  const [rows, setRows] = useState<Row[]>([]);
  const [loading, setLoading] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>("falsePositiveRate");
  const [sortDesc, setSortDesc] = useState(true);

  useEffect(() => {
    let cancelled = false;
    // debt: fija loading síncrono a propósito — el spinner depende de ese
    // flag en el mismo frame en que arranca el fetch. Mismo patrón que
    // red-flags/page.tsx.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true);
    Promise.all(
      rules.map(async (rule) => ({
        ruleId: rule.id,
        name: rule.name,
        stats: await rulesApi.effectiveness(rule.id),
      })),
    )
      .then((results) => {
        if (!cancelled) setRows(results);
      })
      .catch(() => {
        if (!cancelled) toastError("Error al cargar la efectividad de las reglas");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rules]);

  function toggleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDesc((d) => !d);
    } else {
      setSortKey(key);
      setSortDesc(true);
    }
  }

  const sorted = [...rows].sort((a, b) => {
    let cmp: number;
    if (sortKey === "name") {
      cmp = a.name.localeCompare(b.name);
    } else if (sortKey === "total") {
      cmp = a.stats.total - b.stats.total;
    } else if (sortKey === "avgCloseHours") {
      cmp = a.stats.avgCloseHours - b.stats.avgCloseHours;
    } else {
      cmp = a.stats.falsePositiveRate - b.stats.falsePositiveRate;
    }
    return sortDesc ? -cmp : cmp;
  });

  if (loading) {
    return (
      <div className="p-6 text-sm text-muted-foreground">Cargando…</div>
    );
  }

  if (rows.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-12">
          <p className="mb-2 font-medium">Sin datos de efectividad</p>
          <p className="text-sm text-muted-foreground">
            Aparece acá una vez que las reglas generen banderas rojas y se
            cierren casos con disposición.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="rounded-md border overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead className="bg-muted/70">
            <tr>
              <th className="px-3 py-2 text-left">
                <SortHeader
                  label="Regla"
                  sortKey="name"
                  activeKey={sortKey}
                  desc={sortDesc}
                  onSort={toggleSort}
                />
              </th>
              <th className="px-3 py-2 text-right">
                <SortHeader
                  label="Disparos"
                  sortKey="total"
                  activeKey={sortKey}
                  desc={sortDesc}
                  onSort={toggleSort}
                />
              </th>
              <th className="px-3 py-2 text-right">Abiertas</th>
              <th className="px-3 py-2 text-right">
                <SortHeader
                  label="% falsos positivos"
                  sortKey="falsePositiveRate"
                  activeKey={sortKey}
                  desc={sortDesc}
                  onSort={toggleSort}
                />
              </th>
              <th className="px-3 py-2 text-right">
                <SortHeader
                  label="Tiempo medio de cierre"
                  sortKey="avgCloseHours"
                  activeKey={sortKey}
                  desc={sortDesc}
                  onSort={toggleSort}
                />
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border/50">
            {sorted.map((row) => (
              <tr key={row.ruleId} className="hover:bg-muted/30">
                <td className="px-3 py-2">{row.name}</td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {row.stats.total}
                </td>
                <td className="px-3 py-2 text-right tabular-nums text-muted-foreground">
                  {row.stats.open}
                </td>
                <td className="px-3 py-2 text-right tabular-nums font-medium">
                  {row.stats.total - row.stats.open > 0
                    ? `${Math.round(row.stats.falsePositiveRate * 100)}%`
                    : "—"}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {row.stats.total - row.stats.open > 0
                    ? formatHours(row.stats.avgCloseHours)
                    : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function formatHours(hours: number): string {
  if (hours < 24) return `${Math.round(hours)}h`;
  return `${Math.round(hours / 24)}d`;
}
