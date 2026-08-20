"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { useDropzone } from "react-dropzone";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Upload, FileUp, Table, ShieldCheck, Play, Pencil, Search } from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { rulesApi } from "@/lib/api/rules";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import type { Monitor, Rule, SchemaField } from "@/lib/types";
import Link from "next/link";

export default function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [monitor, setMonitor] = useState<Monitor | null>(null);
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<{ recordsIngested: number; evaluationQueued: boolean } | null>(null);
  const [evaluating, setEvaluating] = useState(false);
  const [evalResult, setEvalResult] = useState<{ alertsGenerated: number } | null>(null);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", description: "" });
  const [searchTerm, setSearchTerm] = useState("");
  const { toastError } = useToast();

  useEffect(() => {
    if (id) {
      loadMonitor();
      loadData();
      loadRules();
    }
  }, [id]);

  async function loadMonitor() {
    try { setMonitor(await monitorsApi.get(id)); } catch { toastError("Error al cargar monitor"); }
  }

  async function loadData() {
    try {
      const result = await monitorsApi.getData(id);
      setData(result.data || []);
    } catch { toastError("Error al cargar datos"); }
  }

  async function loadRules() {
    try { setRules(await rulesApi.list(id)); } catch { toastError("Error al cargar reglas"); }
  }

  const onDrop = useCallback(async (acceptedFiles: File[]) => {
    if (acceptedFiles.length === 0) return;
    setUploading(true);
    setUploadResult(null);
    try {
      const result = await monitorsApi.upload(id, acceptedFiles[0]);
      setUploadResult(result);
      loadMonitor();
      loadData();
    } catch { toastError("Error al subir archivo"); } finally {
      setUploading(false);
    }
  }, [id]);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    maxFiles: 1,
    accept: {
      "text/csv": [".csv"],
      "application/json": [".json"],
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": [".xlsx"],
      "application/vnd.ms-excel": [".xls"],
    },
  });

  function openEdit() {
    if (!monitor) return;
    setEditForm({ name: monitor.name, description: monitor.description || "" });
    setIsEditOpen(true);
  }

  async function saveEdit(e: React.FormEvent) {
    e.preventDefault();
    try {
      await monitorsApi.update(id, editForm);
      setIsEditOpen(false);
      loadMonitor();
    } catch { toastError("Error al actualizar monitor"); }
  }

  async function handleEvaluate() {
    setEvaluating(true);
    setEvalResult(null);
    try {
      const result = await monitorsApi.evaluate(id);
      setEvalResult(result);
      loadRules();
    } catch (err) {
      setEvalResult({ alertsGenerated: -1 });
    } finally {
      setEvaluating(false);
    }
  }

  if (!monitor) return null;

  return (
    <>
      <Header title={monitor.name} />
      <div className="p-6">
        <div className="mb-6 flex items-center gap-3">
          <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
            <DialogTrigger asChild>
              <Button size="sm" variant="ghost" onClick={openEdit}>
                <Pencil className="h-4 w-4" />
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader><DialogTitle>Editar monitor</DialogTitle></DialogHeader>
              <form onSubmit={saveEdit} className="space-y-4">
                <div className="space-y-2">
                  <Label>Nombre</Label>
                  <Input value={editForm.name} onChange={(e) => setEditForm({ ...editForm, name: e.target.value })} required />
                </div>
                <div className="space-y-2">
                  <Label>Descripcion</Label>
                  <Textarea value={editForm.description} onChange={(e) => setEditForm({ ...editForm, description: e.target.value })} />
                </div>
                <Button type="submit" className="w-full">Guardar</Button>
              </form>
            </DialogContent>
          </Dialog>
          <Badge variant="outline">{monitor.sourceType.toUpperCase()}</Badge>
          <span className="text-sm text-muted-foreground">{monitor.recordCount} registros</span>
          {monitor.lastIngested && (
            <span className="text-sm text-muted-foreground">
              Ultima carga: {formatDate(monitor.lastIngested)}
            </span>
          )}
          {monitor.recordCount > 0 && (
            <Button size="sm" variant="outline" onClick={handleEvaluate} disabled={evaluating}>
              <Play className="mr-2 h-4 w-4" />
              {evaluating ? "Evaluando..." : "Ejecutar reglas"}
            </Button>
          )}
        </div>

        {evalResult && (
          <div className={`mb-4 rounded-md p-4 ${evalResult.alertsGenerated >= 0 ? "bg-emerald-50 dark:bg-emerald-950" : "bg-red-50 dark:bg-red-950"}`}>
            <p className={`text-sm font-medium ${evalResult.alertsGenerated >= 0 ? "text-emerald-800 dark:text-emerald-200" : "text-red-800 dark:text-red-200"}`}>
              {evalResult.alertsGenerated >= 0
                ? `${evalResult.alertsGenerated} alertas generadas`
                : "Error al evaluar reglas"}
            </p>
          </div>
        )}

        <Tabs defaultValue="upload">
          <TabsList>
            <TabsTrigger value="upload"><Upload className="mr-2 h-4 w-4" />Cargar datos</TabsTrigger>
            <TabsTrigger value="data"><Table className="mr-2 h-4 w-4" />Datos</TabsTrigger>
            <TabsTrigger value="schema"><FileUp className="mr-2 h-4 w-4" />Esquema</TabsTrigger>
            <TabsTrigger value="rules"><ShieldCheck className="mr-2 h-4 w-4" />Reglas</TabsTrigger>
          </TabsList>

          <TabsContent value="upload">
            <Card>
              <CardContent className="pt-6">
                <div
                  {...getRootProps()}
                  className={`flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-12 transition-colors ${
                    isDragActive ? "border-primary bg-primary/5" : "border-muted-foreground/25 hover:border-primary/50"
                  }`}
                >
                  <input {...getInputProps()} />
                  <Upload className="mb-4 h-10 w-10 text-muted-foreground" />
                  {uploading ? (
                    <p className="text-sm text-muted-foreground">Procesando archivo...</p>
                  ) : isDragActive ? (
                    <p className="text-sm">Suelta el archivo aqui</p>
                  ) : (
                    <>
                      <p className="text-sm font-medium">Arrastra un archivo o haz clic para seleccionar</p>
                      <p className="mt-1 text-xs text-muted-foreground">CSV, Excel (.xlsx), JSON</p>
                    </>
                  )}
                </div>

                {uploadResult && (
                  <div className="mt-4 rounded-md bg-emerald-50 p-4 dark:bg-emerald-950">
                    <p className="text-sm font-medium text-emerald-800 dark:text-emerald-200">
                      {uploadResult.recordsIngested} registros cargados
                      {uploadResult.evaluationQueued && " — reglas en evaluacion"}
                    </p>
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="data">
            <Card>
              <CardContent className="pt-6">
                {data.length === 0 ? (
                  <p className="text-center text-sm text-muted-foreground">No hay datos cargados</p>
                ) : (
                  <>
                    <div className="mb-4 flex items-center gap-2">
                      <Search className="h-4 w-4 text-muted-foreground" />
                      <Input
                        placeholder="Buscar en todos los campos..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="max-w-sm"
                      />
                      {searchTerm && (
                        <span className="text-xs text-muted-foreground">
                          {data.filter(row => Object.values(row).some(v => String(v ?? "").toLowerCase().includes(searchTerm.toLowerCase()))).length} resultados
                        </span>
                      )}
                    </div>
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b">
                            {monitor.schema?.map((field: SchemaField) => (
                              <th key={field.name} className="px-3 py-2 text-left font-medium text-muted-foreground">
                                {field.name}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {data
                            .filter(row => !searchTerm || Object.values(row).some(v => String(v ?? "").toLowerCase().includes(searchTerm.toLowerCase())))
                            .slice(0, 100)
                            .map((row, i) => (
                              <tr key={i} className="border-b last:border-0 hover:bg-muted/50">
                                {monitor.schema?.map((field: SchemaField) => {
                                  const val = String(row[field.name] ?? "");
                                  const isMatch = searchTerm && val.toLowerCase().includes(searchTerm.toLowerCase());
                                  return (
                                    <td key={field.name} className={`max-w-[200px] truncate px-3 py-2 ${isMatch ? "bg-yellow-100 dark:bg-yellow-900/30 font-medium" : ""}`}>
                                      {val}
                                    </td>
                                  );
                                })}
                              </tr>
                            ))}
                        </tbody>
                      </table>
                      <p className="mt-2 text-center text-xs text-muted-foreground">
                        Mostrando {Math.min(100, data.filter(row => !searchTerm || Object.values(row).some(v => String(v ?? "").toLowerCase().includes(searchTerm.toLowerCase()))).length)} de {monitor.recordCount} registros
                      </p>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="schema">
            <Card>
              <CardHeader><CardTitle className="text-base">Esquema detectado</CardTitle></CardHeader>
              <CardContent>
                {!monitor.schema || monitor.schema.length === 0 ? (
                  <p className="text-sm text-muted-foreground">Carga datos para detectar el esquema automaticamente</p>
                ) : (
                  <div className="space-y-2">
                    {monitor.schema.map((field: SchemaField) => (
                      <div key={field.name} className="flex items-center justify-between rounded-md border p-3">
                        <div>
                          <p className="text-sm font-medium">{field.name}</p>
                          {field.sample && <p className="text-xs text-muted-foreground">Ejemplo: {field.sample}</p>}
                        </div>
                        <Badge variant="secondary">{field.type}</Badge>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="rules">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Reglas del monitor</CardTitle>
                <Button size="sm" asChild>
                  <Link href={`/rules?monitorId=${id}`}><ShieldCheck className="mr-2 h-4 w-4" />Gestionar reglas</Link>
                </Button>
              </CardHeader>
              <CardContent>
                {rules.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No hay reglas configuradas para este monitor</p>
                ) : (
                  <div className="space-y-2">
                    {rules.map((rule) => (
                      <div key={rule.id} className="flex items-center justify-between rounded-md border p-3">
                        <div>
                          <p className="text-sm font-medium">{rule.name}</p>
                          <p className="text-xs text-muted-foreground">{rule.description}</p>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={rule.active ? "success" : "secondary"}>
                            {rule.active ? "Activa" : "Inactiva"}
                          </Badge>
                          <Badge variant="outline">{rule.triggerCount}x</Badge>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      </div>
    </>
  );
}
