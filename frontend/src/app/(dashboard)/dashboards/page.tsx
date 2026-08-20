"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Plus, BarChart3, Trash2, LayoutGrid } from "lucide-react";
import { dashboardsApi } from "@/lib/api/dashboards";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import type { Dashboard, Monitor } from "@/lib/types";

export default function DashboardsPage() {
  const [dashboards, setDashboards] = useState<Dashboard[]>([]);
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newDashboard, setNewDashboard] = useState({ name: "", description: "", monitorIds: [] as string[] });
  const { toastError } = useToast();

  useEffect(() => {
    loadDashboards();
    monitorsApi.list().then(setMonitors).catch(() => toastError("Error al cargar monitores"));
  }, []);

  async function loadDashboards() {
    try { setDashboards(await dashboardsApi.list()); } catch { toastError("Error al cargar dashboards"); }
  }

  async function createDashboard(e: React.FormEvent) {
    e.preventDefault();
    try {
      await dashboardsApi.create(newDashboard);
      setIsCreateOpen(false);
      setNewDashboard({ name: "", description: "", monitorIds: [] });
      loadDashboards();
    } catch { toastError("Error al crear dashboard"); }
  }

  async function deleteDashboard(id: string) {
    try { await dashboardsApi.delete(id); loadDashboards(); } catch { toastError("Error al eliminar dashboard"); }
  }

  function toggleMonitor(monitorId: string) {
    setNewDashboard((prev) => ({
      ...prev,
      monitorIds: prev.monitorIds.includes(monitorId)
        ? prev.monitorIds.filter((id) => id !== monitorId)
        : [...prev.monitorIds, monitorId],
    }));
  }

  return (
    <>
      <Header title="Dashboards" />
      <div className="p-6">
        <div className="mb-6 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">Visualizaciones personalizadas de tus datos</p>
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button><Plus className="h-4 w-4" /> Nuevo dashboard</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader><DialogTitle>Crear dashboard</DialogTitle></DialogHeader>
              <form onSubmit={createDashboard} className="space-y-4">
                <div className="space-y-2">
                  <Label>Nombre</Label>
                  <Input
                    value={newDashboard.name}
                    onChange={(e) => setNewDashboard({ ...newDashboard, name: e.target.value })}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Descripcion</Label>
                  <Input
                    value={newDashboard.description}
                    onChange={(e) => setNewDashboard({ ...newDashboard, description: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label>Monitores</Label>
                  <div className="space-y-2">
                    {monitors.map((m) => (
                      <label key={m.id} className="flex items-center gap-2 text-sm">
                        <input
                          type="checkbox"
                          checked={newDashboard.monitorIds.includes(m.id)}
                          onChange={() => toggleMonitor(m.id)}
                          className="rounded border-input"
                        />
                        {m.name}
                      </label>
                    ))}
                  </div>
                </div>
                <Button type="submit" className="w-full">Crear</Button>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {dashboards.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <BarChart3 className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="mb-2 font-medium">No hay dashboards</p>
              <p className="text-sm text-muted-foreground">Crea un dashboard para visualizar tus datos</p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {dashboards.map((dashboard) => (
              <Card key={dashboard.id} className="group">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between">
                    <div>
                      <CardTitle className="text-base">
                        <Link href={`/dashboards/${dashboard.id}`} className="hover:underline">
                          {dashboard.name}
                        </Link>
                      </CardTitle>
                      <CardDescription className="mt-1">{dashboard.description}</CardDescription>
                    </div>
                    <LayoutGrid className="h-4 w-4 text-muted-foreground" />
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between text-sm">
                    <div className="flex gap-1">
                      <Badge variant="outline">{dashboard.widgets?.length || 0} widgets</Badge>
                      {dashboard.isPublic && <Badge variant="secondary">Publico</Badge>}
                    </div>
                    <Button variant="ghost" size="icon" className="opacity-0 group-hover:opacity-100" onClick={() => deleteDashboard(dashboard.id)}>
                      <Trash2 className="h-4 w-4 text-destructive" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
