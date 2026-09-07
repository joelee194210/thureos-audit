"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Header } from "@/components/layout/header";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Plus, Database, Upload, Trash2, Copy, Check } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import {
  SourceConfigFields,
  sourceConfigValuesToInput,
} from "@/components/monitors/source-config-fields";
import type {
  Monitor,
  SourceType,
  APIMode,
  APIAuthType,
  CreateSourceConfig,
  SchemaField,
} from "@/lib/types";

export default function MonitorsPage() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newMonitor, setNewMonitor] = useState({
    name: "",
    description: "",
    sourceType: "csv" as SourceType,
    delimiter: "",
    hasHeaderRow: true,
    sheetName: "",
    rootPath: "",
    apiMode: "push" as APIMode,
    pullUrl: "",
    pullMethod: "GET",
    pullAuthType: "none" as APIAuthType,
    pullAuthHeaderName: "",
    pullAuthValue: "",
    pullIntervalMinutes: "60",
  });
  const [detectedSchema, setDetectedSchema] = useState<SchemaField[] | null>(
    null,
  );
  const [pushTokenReveal, setPushTokenReveal] = useState<{
    url: string;
    token: string;
  } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const { toastError } = useToast();

  useEffect(() => {
    loadMonitors();
  }, []);

  async function loadMonitors() {
    try {
      const data = await monitorsApi.list();
      setMonitors(data);
    } catch {
      toastError("Error al cargar monitores");
    }
  }

  async function createMonitor(e: React.FormEvent) {
    e.preventDefault();
    try {
      const sourceConfig = sourceConfigValuesToInput(
        newMonitor.sourceType,
        newMonitor,
      ) as CreateSourceConfig | undefined;

      const result = await monitorsApi.create({
        name: newMonitor.name,
        description: newMonitor.description,
        sourceType: newMonitor.sourceType,
        sourceConfig,
        schema: detectedSchema ?? undefined,
      });

      setIsCreateOpen(false);
      if ("pushToken" in result) {
        const apiUrl =
          process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
        setPushTokenReveal({
          url: `${apiUrl}/ingest/${result.monitor.id}`,
          token: result.pushToken,
        });
        setTokenCopied(false);
      }
      setNewMonitor({
        name: "",
        description: "",
        sourceType: "csv",
        delimiter: "",
        hasHeaderRow: true,
        sheetName: "",
        rootPath: "",
        apiMode: "push",
        pullUrl: "",
        pullMethod: "GET",
        pullAuthType: "none",
        pullAuthHeaderName: "",
        pullAuthValue: "",
        pullIntervalMinutes: "60",
      });
      setDetectedSchema(null);
      loadMonitors();
    } catch {
      toastError("Error al crear monitor");
    }
  }

  async function copyPushToken() {
    if (!pushTokenReveal) return;
    await navigator.clipboard.writeText(
      `URL: ${pushTokenReveal.url}\nHeader: X-Ingest-Token: ${pushTokenReveal.token}`,
    );
    setTokenCopied(true);
  }

  async function deleteMonitor(id: string) {
    try {
      await monitorsApi.delete(id);
      loadMonitors();
    } catch {
      toastError("Error al eliminar monitor");
    }
  }

  return (
    <>
      <Header title="Monitores" />
      <div className="p-6">
        <div className="mb-6 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            Gestiona tus fuentes de datos y esquemas
          </p>
          <Dialog
            open={isCreateOpen}
            onOpenChange={(open) => {
              setIsCreateOpen(open);
              if (!open) setDetectedSchema(null);
            }}
          >
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
                    onChange={(e) =>
                      setNewMonitor({ ...newMonitor, name: e.target.value })
                    }
                    placeholder="Ej: Tarjetas de credito"
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Descripción</Label>
                  <Textarea
                    value={newMonitor.description}
                    onChange={(e) =>
                      setNewMonitor({
                        ...newMonitor,
                        description: e.target.value,
                      })
                    }
                    placeholder="Describe que datos va a monitorear"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Tipo de fuente</Label>
                  <Select
                    value={newMonitor.sourceType}
                    onValueChange={(v) => {
                      setNewMonitor({
                        ...newMonitor,
                        sourceType: v as SourceType,
                      });
                      setDetectedSchema(null);
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="csv">CSV</SelectItem>
                      <SelectItem value="excel">Excel</SelectItem>
                      <SelectItem value="json">JSON</SelectItem>
                      <SelectItem value="txt">TXT</SelectItem>
                      <SelectItem value="api">API</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <SourceConfigFields
                  sourceType={newMonitor.sourceType}
                  values={newMonitor}
                  onChange={(patch) =>
                    setNewMonitor({ ...newMonitor, ...patch })
                  }
                  onSchemaDetected={setDetectedSchema}
                />

                <Button type="submit" className="w-full">
                  Crear
                </Button>
              </form>
            </DialogContent>
          </Dialog>

          <Dialog
            open={!!pushTokenReveal}
            onOpenChange={(open) => !open && setPushTokenReveal(null)}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Credenciales de ingesta</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Guarda esta URL y este token ahora — no se van a volver a
                  mostrar.
                </p>
                <div className="space-y-2">
                  <Label>URL</Label>
                  <Input
                    readOnly
                    value={pushTokenReveal?.url ?? ""}
                    className="font-mono text-xs"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Header: X-Ingest-Token</Label>
                  <Input
                    readOnly
                    value={pushTokenReveal?.token ?? ""}
                    className="font-mono text-xs"
                  />
                </div>
                <Button
                  onClick={copyPushToken}
                  className="w-full"
                  variant="outline"
                >
                  {tokenCopied ? (
                    <Check className="mr-2 h-4 w-4" />
                  ) : (
                    <Copy className="mr-2 h-4 w-4" />
                  )}
                  {tokenCopied ? "URL y token copiados" : "Copiar URL y token"}
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        </div>

        {monitors.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <Database className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="mb-2 font-medium">No hay monitores</p>
              <p className="text-sm text-muted-foreground">
                Crea tu primer monitor para empezar
              </p>
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
                        <Link
                          href={`/monitors/${monitor.id}`}
                          className="hover:underline"
                        >
                          {monitor.name}
                        </Link>
                      </CardTitle>
                      <CardDescription className="mt-1">
                        {monitor.description}
                      </CardDescription>
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
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="ghost" size="icon">
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              ¿Eliminar &quot;{monitor.name}&quot;?
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              Se elimina la definición del monitor y deja de
                              recibir datos nuevos. Los {monitor.recordCount}{" "}
                              registros ya ingeridos y las reglas asociadas
                              <strong> no se borran</strong> — quedan huérfanos,
                              sin un monitor que los vincule, por retención
                              regulatoria. Esta acción no se puede deshacer
                              desde aquí.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancelar</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => deleteMonitor(monitor.id)}
                            >
                              Eliminar
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
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
