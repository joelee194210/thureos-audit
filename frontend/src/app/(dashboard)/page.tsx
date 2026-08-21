"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Monitor, ShieldCheck, Bell, BarChart3 } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { rulesApi } from "@/lib/api/rules";
import { redFlagsApi } from "@/lib/api/red-flags";
import { dashboardsApi } from "@/lib/api/dashboards";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor as MonitorType, RedFlag, Severity } from "@/lib/types";

const severityColors: Record<Severity, "destructive" | "warning" | "secondary" | "default"> = {
  critical: "destructive",
  high: "destructive",
  medium: "warning",
  low: "secondary",
};

export default function HomePage() {
  const [stats, setStats] = useState({
    monitors: 0,
    rules: 0,
    redFlags: 0,
    dashboards: 0,
  });
  const [recentRedFlags, setRecentRedFlags] = useState<RedFlag[]>([]);
  const [recentMonitors, setRecentMonitors] = useState<MonitorType[]>([]);
  const { toastError } = useToast();

  useEffect(() => {
    Promise.all([
      monitorsApi.list(),
      rulesApi.list(),
      redFlagsApi.stats(),
      dashboardsApi.list(),
      redFlagsApi.list({ limit: 5 }),
    ]).then(([monitors, rules, redFlagStats, dashboards, redFlags]) => {
      setStats({
        monitors: monitors.length,
        rules: rules.length,
        redFlags: redFlagStats.new,
        dashboards: dashboards.length,
      });
      setRecentRedFlags(redFlags);
      setRecentMonitors(monitors.slice(0, 5));
    }).catch(() => toastError("Error al cargar datos del dashboard"));
  }, []);

  const statCards = [
    { title: "Monitores", value: stats.monitors, icon: Monitor, color: "text-accent-fg" },
    { title: "Reglas activas", value: stats.rules, icon: ShieldCheck, color: "text-success-fg" },
    { title: "Banderas rojas nuevas", value: stats.redFlags, icon: Bell, color: "text-warning-fg" },
    { title: "Dashboards", value: stats.dashboards, icon: BarChart3, color: "text-chart-4" },
  ];

  return (
    <>
      <Header title="Dashboard" />
      <div className="space-y-6 p-6">
        {/* Stats */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {statCards.map((stat) => (
            <Card key={stat.title}>
              <CardContent className="flex items-center gap-4 p-6">
                <div className={`rounded-lg bg-muted p-3 ${stat.color}`}>
                  <stat.icon className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-sm text-muted-foreground">{stat.title}</p>
                  <p className="text-2xl font-bold">{stat.value}</p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="grid gap-6 lg:grid-cols-2">
          {/* Recent Red Flags */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Banderas rojas recientes</CardTitle>
            </CardHeader>
            <CardContent>
              {recentRedFlags.length === 0 ? (
                <p className="text-sm text-muted-foreground">No hay banderas rojas</p>
              ) : (
                <div className="space-y-3">
                  {recentRedFlags.map((redFlag) => (
                    <div key={redFlag.id} className="flex items-start justify-between gap-2 rounded-md border p-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{redFlag.ruleName}</p>
                        <p className="text-xs text-muted-foreground">{redFlag.monitorName}</p>
                        <p className="mt-1 text-xs text-muted-foreground">{formatDate(redFlag.createdAt)}</p>
                      </div>
                      <Badge variant={severityColors[redFlag.severity]}>{redFlag.severity}</Badge>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Recent Monitors */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Monitores</CardTitle>
            </CardHeader>
            <CardContent>
              {recentMonitors.length === 0 ? (
                <p className="text-sm text-muted-foreground">No hay monitores configurados</p>
              ) : (
                <div className="space-y-3">
                  {recentMonitors.map((monitor) => (
                    <div key={monitor.id} className="flex items-center justify-between rounded-md border p-3">
                      <div>
                        <p className="text-sm font-medium">{monitor.name}</p>
                        <p className="text-xs text-muted-foreground">
                          {monitor.recordCount} registros &middot; {monitor.sourceType.toUpperCase()}
                        </p>
                      </div>
                      <Badge variant="outline">{monitor.sourceType}</Badge>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
