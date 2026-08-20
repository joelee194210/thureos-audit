"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Plus, Database, Upload, Trash2 } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor, SourceType } from "@/lib/types";

export default function MonitorsPage() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newMonitor, setNewMonitor] = useState({ name: "", description: "", sourceType: "csv" as SourceType });
  const { toastError } = useToast();

  useEffect(() => {
    loadMonitors();
  }, []);

  async function loadMonitors() {
    try {
      const data = await monitorsApi.list();
      setMonitors(data);
    } catch { toastError("Error al cargar monitores"); }
  }

  async function createMonitor(e: React.FormEvent) {
    e.preventDefault();
    try {
      await monitorsApi.create(newMonitor);
      setIsCreateOpen(false);
      setNewMonitor({ name: "", description: "", sourceType: "csv" });
      loadMonitors();
    } catch { toastError("Error al crear monitor"); }
  }

  async function deleteMonitor(id: string) {
    try {
      await monitorsApi.delete(id);
      loadMonitors();
    } catch { toastError("Error al eliminar monitor"); }
  }

  return (
    <>
      <Header title="Monitores" />
      <div className="p-6">
        <div className="mb-6 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Gestiona tus fuentes de datos y esquemas
          </p>
          <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4" /> Nuevo monitor
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Crear monitor</DialogTitle>
              </DialogHeader>
              <form onSubmit={createMonitor} className="space-y-4">
                <div className="space-y-2">
                  <Label>Nombre</Label>
                  <Input
                    value={newMonitor.name}
                    onChange={(e) => setNewMonitor({ ...newMonitor, name: e.target.value })}
                    placeholder="Ej: Tarjetas de credito"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Descripción</Label>
                  <Textarea
                    value={newMonitor.description}
                    onChange={(e) => setNewMonitor({ ...newMonitor, description: e.target.value })}
                    placeholder="Describe que datos va a monitorear"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Tipo de fuente</Label>
                  <Select
                    value={newMonitor.sourceType}
                    onValueChange={(v) => setNewMonitor({ ...newMonitor, sourceType: v as SourceType })}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="csv">CSV</SelectItem>
                      <SelectItem value="excel">Excel</SelectItem>
                      <SelectItem value="json">JSON</SelectItem>
                      <SelectItem value="api">API</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <Button type="submit" className="w-full">Crear</Button>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {monitors.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Database className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="mb-2 font-medium">No hay monitores</p>
              <p className="text-sm text-muted-foreground">Crea tu primer monitor para empezar</p>
            </CardContent>
          </Card>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {monitors.map((monitor) => (
              <Card key={monitor.id} className="group">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between">
                    <div>
                      <CardTitle className="text-base">
                        <Link href={`/monitors/${monitor.id}`} className="hover:underline">
                          {monitor.name}
                        </Link>
                      </CardTitle>
                      <CardDescription className="mt-1">{monitor.description}</CardDescription>
                    </div>
                    <Badge variant="outline">{monitor.sourceType}</Badge>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between text-sm">
                    <div className="space-y-1">
                      <p className="text-muted-foreground">
                        {monitor.recordCount} registros
                      </p>
                      {monitor.lastIngested && (
                        <p className="text-xs text-muted-foreground">
                          Última carga: {formatDate(monitor.lastIngested)}
                        </p>
                      )}
                    </div>
                    <div className="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                      <Button variant="ghost" size="icon" asChild>
                        <Link href={`/monitors/${monitor.id}`}>
                          <Upload className="h-4 w-4" />
                        </Link>
                      </Button>
                      <Button variant="ghost" size="icon" onClick={() => deleteMonitor(monitor.id)}>
                        <Trash2 className="h-4 w-4 text-destructive" />
                      </Button>
                    </div>
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
