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
import { Switch } from "@/components/ui/switch";
import { Plus, Database, Upload, Trash2, Copy, Check } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor, SourceType, APIMode, APIAuthType, CreateSourceConfig } from "@/lib/types";

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
  const [pushTokenReveal, setPushTokenReveal] = useState<{ url: string; token: string } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
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
      let sourceConfig: CreateSourceConfig | undefined;

      if (newMonitor.sourceType === "csv" || newMonitor.sourceType === "txt") {
        sourceConfig = {
          delimiter: newMonitor.delimiter || undefined,
          hasHeaderRow: newMonitor.hasHeaderRow,
        };
      } else if (newMonitor.sourceType === "excel") {
        sourceConfig = newMonitor.sheetName ? { sheetName: newMonitor.sheetName } : undefined;
      } else if (newMonitor.sourceType === "json") {
        sourceConfig = newMonitor.rootPath ? { rootPath: newMonitor.rootPath } : undefined;
      } else if (newMonitor.sourceType === "api") {
        sourceConfig =
          newMonitor.apiMode === "pull"
            ? {
                mode: "pull",
                pullUrl: newMonitor.pullUrl,
                pullMethod: newMonitor.pullMethod,
                pullAuthType: newMonitor.pullAuthType,
                pullAuthHeaderName: newMonitor.pullAuthHeaderName || undefined,
                pullAuthValue: newMonitor.pullAuthValue || undefined,
                pullIntervalMinutes: Number(newMonitor.pullIntervalMinutes) || 60,
                rootPath: newMonitor.rootPath || undefined,
              }
            : { mode: "push", rootPath: newMonitor.rootPath || undefined };
      }

      const result = await monitorsApi.create({
        name: newMonitor.name,
        description: newMonitor.description,
        sourceType: newMonitor.sourceType,
        sourceConfig,
      });

      setIsCreateOpen(false);
      if ("pushToken" in result) {
        const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
        setPushTokenReveal({ url: `${apiUrl}/ingest/${result.monitor.id}`, token: result.pushToken });
        setTokenCopied(false);
      }
      setNewMonitor({
        name: "", description: "", sourceType: "csv",
        delimiter: "", hasHeaderRow: true, sheetName: "", rootPath: "",
        apiMode: "push", pullUrl: "", pullMethod: "GET", pullAuthType: "none",
        pullAuthHeaderName: "", pullAuthValue: "", pullIntervalMinutes: "60",
      });
      loadMonitors();
    } catch { toastError("Error al crear monitor"); }
  }

  async function copyPushToken() {
    if (!pushTokenReveal) return;
    await navigator.clipboard.writeText(pushTokenReveal.token);
    setTokenCopied(true);
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
                      <SelectItem value="txt">TXT</SelectItem>
                      <SelectItem value="api">API</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {(newMonitor.sourceType === "csv" || newMonitor.sourceType === "txt") && (
                  <div className="space-y-3 rounded-md border p-3">
                    <div className="space-y-2">
                      <Label>Delimitador</Label>
                      <Input
                        value={newMonitor.delimiter}
                        onChange={(e) => setNewMonitor({ ...newMonitor, delimiter: e.target.value.slice(0, 1) })}
                        placeholder={newMonitor.sourceType === "txt" ? "Tab (por defecto)" : "coma (por defecto)"}
                        maxLength={1}
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <Label>Primera fila es encabezado</Label>
                      <Switch
                        checked={newMonitor.hasHeaderRow}
                        onCheckedChange={(v) => setNewMonitor({ ...newMonitor, hasHeaderRow: v })}
                      />
                    </div>
                  </div>
                )}

                {newMonitor.sourceType === "excel" && (
                  <div className="space-y-2 rounded-md border p-3">
                    <Label>Nombre de la hoja</Label>
                    <Input
                      value={newMonitor.sheetName}
                      onChange={(e) => setNewMonitor({ ...newMonitor, sheetName: e.target.value })}
                      placeholder="Primera hoja (por defecto)"
                    />
                  </div>
                )}

                {newMonitor.sourceType === "json" && (
                  <div className="space-y-2 rounded-md border p-3">
                    <Label>Ruta raíz (opcional)</Label>
                    <Input
                      value={newMonitor.rootPath}
                      onChange={(e) => setNewMonitor({ ...newMonitor, rootPath: e.target.value })}
                      placeholder="ej. data.records — vacío usa la raíz del JSON"
                    />
                  </div>
                )}

                {newMonitor.sourceType === "api" && (
                  <div className="space-y-3 rounded-md border p-3">
                    <div className="space-y-2">
                      <Label>Modo</Label>
                      <Select
                        value={newMonitor.apiMode}
                        onValueChange={(v) => setNewMonitor({ ...newMonitor, apiMode: v as APIMode })}
                      >
                        <SelectTrigger><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectItem value="push">Push — el sistema externo nos envía datos</SelectItem>
                          <SelectItem value="pull">Pull — consultamos una API externa por horario</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-2">
                      <Label>Ruta raíz (opcional)</Label>
                      <Input
                        value={newMonitor.rootPath}
                        onChange={(e) => setNewMonitor({ ...newMonitor, rootPath: e.target.value })}
                        placeholder="ej. data.records — si la API envuelve el array en un campo"
                      />
                    </div>

                    {newMonitor.apiMode === "push" ? (
                      <p className="text-xs text-muted-foreground">
                        Al crear el monitor se genera una URL y un token — se muestran una sola vez.
                      </p>
                    ) : (
                      <>
                        <div className="space-y-2">
                          <Label>URL a consultar</Label>
                          <Input
                            value={newMonitor.pullUrl}
                            onChange={(e) => setNewMonitor({ ...newMonitor, pullUrl: e.target.value })}
                            placeholder="https://api.ejemplo.com/transacciones"
                            required
                          />
                        </div>
                        <div className="grid grid-cols-2 gap-3">
                          <div className="space-y-2">
                            <Label>Método</Label>
                            <Select
                              value={newMonitor.pullMethod}
                              onValueChange={(v) => setNewMonitor({ ...newMonitor, pullMethod: v })}
                            >
                              <SelectTrigger><SelectValue /></SelectTrigger>
                              <SelectContent>
                                <SelectItem value="GET">GET</SelectItem>
                                <SelectItem value="POST">POST</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>
                          <div className="space-y-2">
                            <Label>Cada (minutos)</Label>
                            <Input
                              type="number"
                              min={1}
                              value={newMonitor.pullIntervalMinutes}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullIntervalMinutes: e.target.value })}
                            />
                          </div>
                        </div>
                        <div className="space-y-2">
                          <Label>Autenticación</Label>
                          <Select
                            value={newMonitor.pullAuthType}
                            onValueChange={(v) => setNewMonitor({ ...newMonitor, pullAuthType: v as APIAuthType })}
                          >
                            <SelectTrigger><SelectValue /></SelectTrigger>
                            <SelectContent>
                              <SelectItem value="none">Ninguna</SelectItem>
                              <SelectItem value="api_key_header">API key (header)</SelectItem>
                              <SelectItem value="bearer">Bearer token</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>
                        {newMonitor.pullAuthType === "api_key_header" && (
                          <div className="space-y-2">
                            <Label>Nombre del header</Label>
                            <Input
                              value={newMonitor.pullAuthHeaderName}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullAuthHeaderName: e.target.value })}
                              placeholder="ej. X-API-Key"
                            />
                          </div>
                        )}
                        {newMonitor.pullAuthType !== "none" && (
                          <div className="space-y-2">
                            <Label>{newMonitor.pullAuthType === "bearer" ? "Token" : "Valor de la API key"}</Label>
                            <Input
                              type="password"
                              value={newMonitor.pullAuthValue}
                              onChange={(e) => setNewMonitor({ ...newMonitor, pullAuthValue: e.target.value })}
                            />
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}

                <Button type="submit" className="w-full">Crear</Button>
              </form>
            </DialogContent>
          </Dialog>

          <Dialog open={!!pushTokenReveal} onOpenChange={(open) => !open && setPushTokenReveal(null)}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Credenciales de ingesta</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Guarda esta URL y este token ahora — no se van a volver a mostrar.
                </p>
                <div className="space-y-2">
                  <Label>URL</Label>
                  <Input readOnly value={pushTokenReveal?.url ?? ""} className="font-mono text-xs" />
                </div>
                <div className="space-y-2">
                  <Label>Header: X-Ingest-Token</Label>
                  <Input readOnly value={pushTokenReveal?.token ?? ""} className="font-mono text-xs" />
                </div>
                <Button onClick={copyPushToken} className="w-full" variant="outline">
                  {tokenCopied ? <Check className="mr-2 h-4 w-4" /> : <Copy className="mr-2 h-4 w-4" />}
                  {tokenCopied ? "Token copiado" : "Copiar token"}
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
