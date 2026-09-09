"use client";

import { useEffect, useState, useCallback } from "react";
import { useDropzone } from "react-dropzone";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Upload,
  FileUp,
  Table,
  ShieldCheck,
  Play,
  Pencil,
  Search,
  KeyRound,
  Copy,
  Check,
  AlertTriangle,
} from "lucide-react";
import { monitorsApi } from "@/lib/api/monitors";
import { rulesApi } from "@/lib/api/rules";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import {
  SourceConfigFields,
  sourceConfigValuesToInput,
  type SourceConfigValues,
} from "@/components/monitors/source-config-fields";
import { DerivedTimestampCard } from "@/components/monitors/derived-timestamp-card";
import { STATUS_CLASSES, STATUS_FG } from "@/lib/semantic-colors";
import type {
  Monitor,
  Rule,
  SchemaField,
  CreateSourceConfig,
  UploadCheckResult,
} from "@/lib/types";
import Link from "next/link";
import { useTabStore } from "@/stores/tab-store";
import { useAuthStore } from "@/stores/auth-store";

export function MonitorTabContent({
  params,
  tabId,
}: {
  params: Record<string, string>;
  tabId: string;
}) {
  const { id } = params;
  const [monitor, setMonitor] = useState<Monitor | null>(null);
  const [schemaEdits, setSchemaEdits] = useState<SchemaField[]>([]);
  const [savingSchema, setSavingSchema] = useState(false);
  const [data, setData] = useState<Record<string, unknown>[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState<{
    recordsIngested: number;
    evaluationQueued: boolean;
  } | null>(null);
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [checkResult, setCheckResult] = useState<UploadCheckResult | null>(
    null,
  );
  const [checking, setChecking] = useState(false);
  const [evaluating, setEvaluating] = useState(false);
  const [evalResult, setEvalResult] = useState<{
    redFlagsGenerated: number;
  } | null>(null);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [editForm, setEditForm] = useState<
    { name: string; description: string } & SourceConfigValues
  >({
    name: "",
    description: "",
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
  const [searchTerm, setSearchTerm] = useState("");
  const [rotatingToken, setRotatingToken] = useState(false);
  const [tokenReveal, setTokenReveal] = useState<{
    url: string;
    token: string;
  } | null>(null);
  const [tokenCopied, setTokenCopied] = useState(false);
  const { toastError, toastSuccess } = useToast();
  const updateTabLabel = useTabStore((s) => s.updateTabLabel);
  const registerEntityLabel = useTabStore((s) => s.registerEntityLabel);
  const user = useAuthStore((s) => s.user);

  useEffect(() => {
    if (monitor?.name) {
      updateTabLabel(tabId, monitor.name);
      registerEntityLabel(id, monitor.name);
    }
  }, [monitor?.name, tabId, id, updateTabLabel, registerEntityLabel]);

  useEffect(() => {
    if (id) {
      loadMonitor();
      loadData();
      loadRules();
    }
  }, [id]);

  useEffect(() => {
    if (monitor?.schema) {
      // debt: sincroniza el estado editable con monitor.schema al cargar
      // o recargar el monitor (y tras guardar el esquema) — mismo tipo de
      // deuda de react-hooks/set-state-in-effect que el resto del archivo
      // (ver el useEffect de carga inicial, más arriba), aunque ese caso
      // es un loader async y este es un sync directo de estado.
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSchemaEdits(monitor.schema);
    }
  }, [monitor?.schema]);

  async function loadMonitor() {
    try {
      setMonitor(await monitorsApi.get(id));
    } catch {
      toastError("Error al cargar monitor");
    }
  }

  async function loadData() {
    try {
      const result = await monitorsApi.getData(id);
      setData(result.data || []);
    } catch {
      toastError("Error al cargar datos");
    }
  }

  async function loadRules() {
    try {
      setRules(await rulesApi.list(id));
    } catch {
      toastError("Error al cargar reglas");
    }
  }

  function updateImpliedDecimals(fieldName: string, value: number) {
    setSchemaEdits((prev) =>
      prev.map((f) => (f.name === fieldName ? { ...f, impliedDecimals: value } : f)),
    );
  }

  function updateDateFormat(fieldName: string, value: string) {
    setSchemaEdits((prev) =>
      prev.map((f) => (f.name === fieldName ? { ...f, dateFormat: value } : f)),
    );
  }

  async function handleSaveSchema() {
    setSavingSchema(true);
    try {
      const result = await monitorsApi.updateSchema(id, schemaEdits);
      setMonitor((prev) => (prev ? { ...prev, schema: result.schema } : prev));
      toastSuccess("Esquema actualizado");
    } catch {
      toastError("No se pudo guardar el esquema");
    } finally {
      setSavingSchema(false);
    }
  }

  const onDrop = useCallback(
    async (acceptedFiles: File[]) => {
      if (acceptedFiles.length === 0) return;
      const file = acceptedFiles[0];
      setChecking(true);
      try {
        const check = await monitorsApi.uploadCheck(id, file);
        if (!check.match || check.rowsThatWouldFail > 0) {
          setPendingFile(file);
          setCheckResult(check);
          return;
        }
        await doUpload(file);
      } catch {
        // Si la verificación misma falla (ej. red), se intenta subir
        // directo — el endpoint real vuelve a validar de todas formas.
        await doUpload(file);
      } finally {
        setChecking(false);
      }
    },
    [id],
  );

  async function doUpload(file: File) {
    setUploading(true);
    setUploadResult(null);
    try {
      const result = await monitorsApi.upload(id, file);
      setUploadResult(result);
      loadMonitor();
      loadData();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al subir archivo");
    } finally {
      setUploading(false);
    }
  }

  async function confirmUploadAnyway() {
    if (!pendingFile) return;
    const file = pendingFile;
    setPendingFile(null);
    setCheckResult(null);
    await doUpload(file);
  }

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    maxFiles: 1,
    accept: {
      "text/csv": [".csv"],
      "application/json": [".json"],
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": [
        ".xlsx",
      ],
      "application/vnd.ms-excel": [".xls"],
      "text/plain": [".txt"],
    },
  });

  function openEdit() {
    if (!monitor) return;
    const cfg = monitor.sourceConfig;
    setEditForm({
      name: monitor.name,
      description: monitor.description || "",
      delimiter: cfg?.delimiter ?? "",
      hasHeaderRow: cfg?.hasHeaderRow ?? true,
      sheetName: cfg?.sheetName ?? "",
      rootPath: cfg?.rootPath ?? "",
      apiMode: cfg?.mode ?? "push",
      pullUrl: cfg?.pullUrl ?? "",
      pullMethod: cfg?.pullMethod ?? "GET",
      pullAuthType: cfg?.pullAuthType ?? "none",
      pullAuthHeaderName: cfg?.pullAuthHeaderName ?? "",
      pullAuthValue: "",
      pullIntervalMinutes: String(cfg?.pullIntervalMinutes ?? 60),
    });
    setIsEditOpen(true);
  }

  async function saveEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!monitor) return;
    try {
      const sourceConfig = sourceConfigValuesToInput(
        monitor.sourceType,
        editForm,
        { alwaysInclude: true },
      ) as CreateSourceConfig | undefined;
      await monitorsApi.update(id, {
        name: editForm.name,
        description: editForm.description,
        sourceConfig,
      });
      setIsEditOpen(false);
      loadMonitor();
    } catch {
      toastError("Error al actualizar monitor");
    }
  }

  async function rotateToken() {
    setRotatingToken(true);
    try {
      const result = await monitorsApi.rotatePushToken(id);
      const apiUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
      setTokenReveal({
        url: `${apiUrl}/ingest/${id}`,
        token: result.pushToken,
      });
      setTokenCopied(false);
    } catch {
      toastError("Error al rotar el token");
    } finally {
      setRotatingToken(false);
    }
  }

  async function copyTokenReveal() {
    if (!tokenReveal) return;
    await navigator.clipboard.writeText(
      `URL: ${tokenReveal.url}\nHeader: X-Ingest-Token: ${tokenReveal.token}`,
    );
    setTokenCopied(true);
  }

  async function handleEvaluate() {
    setEvaluating(true);
    setEvalResult(null);
    try {
      const result = await monitorsApi.evaluate(id);
      setEvalResult(result);
      loadRules();
    } catch {
      setEvalResult({ redFlagsGenerated: -1 });
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
              <DialogHeader>
                <DialogTitle>Editar monitor</DialogTitle>
              </DialogHeader>
              <form onSubmit={saveEdit} className="space-y-4">
                <div className="space-y-2">
                  <Label>Nombre</Label>
                  <Input
                    value={editForm.name}
                    onChange={(e) =>
                      setEditForm({ ...editForm, name: e.target.value })
                    }
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label>Descripción</Label>
                  <Textarea
                    value={editForm.description}
                    onChange={(e) =>
                      setEditForm({ ...editForm, description: e.target.value })
                    }
                  />
                </div>
                <SourceConfigFields
                  sourceType={monitor.sourceType}
                  values={editForm}
                  onChange={(patch) => setEditForm({ ...editForm, ...patch })}
                  editing
                />
                <Button type="submit" className="w-full">
                  Guardar
                </Button>
              </form>
            </DialogContent>
          </Dialog>

          <Dialog
            open={!!tokenReveal}
            onOpenChange={(open) => !open && setTokenReveal(null)}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Nuevo token de ingesta</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <p className="text-sm text-muted-foreground">
                  Guarda esta URL y este token ahora — el token anterior ya dejó
                  de funcionar y este no se vuelve a mostrar.
                </p>
                <div className="space-y-2">
                  <Label>URL</Label>
                  <Input
                    readOnly
                    value={tokenReveal?.url ?? ""}
                    className="font-mono text-xs"
                  />
                </div>
                <div className="space-y-2">
                  <Label>Header: X-Ingest-Token</Label>
                  <Input
                    readOnly
                    value={tokenReveal?.token ?? ""}
                    className="font-mono text-xs"
                  />
                </div>
                <Button
                  onClick={copyTokenReveal}
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
          <Badge variant="outline">{monitor.sourceType.toUpperCase()}</Badge>
          <span className="text-sm text-muted-foreground">
            {monitor.recordCount} registros
          </span>
          {monitor.lastIngested && (
            <span className="text-sm text-muted-foreground">
              Última carga: {formatDate(monitor.lastIngested)}
            </span>
          )}
          {monitor.recordCount > 0 && (
            <Button
              size="sm"
              variant="outline"
              onClick={handleEvaluate}
              disabled={evaluating}
            >
              <Play className="mr-2 h-4 w-4" />
              {evaluating ? "Evaluando..." : "Ejecutar reglas"}
            </Button>
          )}
        </div>

        {evalResult && (
          <div
            className={`mb-4 rounded-md p-4 ${evalResult.redFlagsGenerated >= 0 ? "bg-success-bg" : "bg-danger-bg"}`}
          >
            <p
              className={`text-sm font-medium ${evalResult.redFlagsGenerated >= 0 ? "text-success-fg" : "text-danger-fg"}`}
            >
              {evalResult.redFlagsGenerated >= 0
                ? `${evalResult.redFlagsGenerated} banderas rojas generadas`
                : "Error al evaluar reglas"}
            </p>
          </div>
        )}

        <Tabs defaultValue="upload">
          <TabsList>
            <TabsTrigger value="upload">
              <Upload className="mr-2 h-4 w-4" />
              Cargar datos
            </TabsTrigger>
            <TabsTrigger value="data">
              <Table className="mr-2 h-4 w-4" />
              Datos
            </TabsTrigger>
            <TabsTrigger value="schema">
              <FileUp className="mr-2 h-4 w-4" />
              Esquema
            </TabsTrigger>
            <TabsTrigger value="rules">
              <ShieldCheck className="mr-2 h-4 w-4" />
              Reglas
            </TabsTrigger>
          </TabsList>

          <TabsContent value="upload">
            <Card>
              <CardContent className="pt-6">
                {monitor.sourceType === "api" ? (
                  !monitor.sourceConfig ? (
                    <div
                      className={`flex items-start gap-3 rounded-md border p-4 ${STATUS_CLASSES.warning}`}
                    >
                      <AlertTriangle
                        className={`mt-0.5 h-5 w-5 shrink-0 ${STATUS_FG.warning}`}
                      />
                      <div>
                        <p
                          className={`text-sm font-medium ${STATUS_FG.warning}`}
                        >
                          Monitor sin configurar
                        </p>
                        <p className={`mt-1 text-sm ${STATUS_FG.warning}`}>
                          Este monitor es de tipo API pero no tiene un modo
                          (push o pull) configurado, así que no puede recibir
                          datos todavía. Usa &quot;Editar&quot; para
                          configurarlo.
                        </p>
                      </div>
                    </div>
                  ) : monitor.sourceConfig.mode === "pull" ? (
                    <div className="space-y-3">
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">
                          URL
                        </span>
                        <span className="max-w-[60%] truncate text-sm font-mono">
                          {monitor.sourceConfig?.pullUrl}
                        </span>
                      </div>
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">
                          Cada
                        </span>
                        <span className="text-sm">
                          {monitor.sourceConfig?.pullIntervalMinutes ?? 60}{" "}
                          minutos
                        </span>
                      </div>
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">
                          Próxima consulta
                        </span>
                        <span className="text-sm">
                          {monitor.sourceConfig?.nextPullAt
                            ? formatDate(monitor.sourceConfig.nextPullAt)
                            : "Pendiente de programar"}
                        </span>
                      </div>
                      <div className="flex items-center justify-between rounded-md border p-3">
                        <span className="text-sm text-muted-foreground">
                          Último intento
                        </span>
                        <span className="text-sm">
                          {monitor.sourceConfig?.lastPullAt
                            ? formatDate(monitor.sourceConfig.lastPullAt)
                            : "Aún no se ha ejecutado"}
                        </span>
                      </div>
                      {monitor.sourceConfig?.lastPullStatus === "error" && (
                        <div className="rounded-md bg-danger-bg p-3 text-sm text-danger-fg">
                          {monitor.sourceConfig.lastPullError}
                        </div>
                      )}
                      {monitor.sourceConfig?.lastPullStatus === "ok" && (
                        <div className="rounded-md bg-success-bg p-3 text-sm text-success-fg">
                          Última consulta exitosa
                        </div>
                      )}
                    </div>
                  ) : (
                    <div className="space-y-3">
                      <p className="text-sm text-muted-foreground">
                        Este monitor recibe datos por push. La URL y el token se
                        mostraron una sola vez al crearlo — si los perdiste,
                        rota el token para generar uno nuevo (el anterior deja
                        de funcionar de inmediato).
                      </p>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={rotatingToken}
                          >
                            <KeyRound className="mr-2 h-4 w-4" />
                            {rotatingToken ? "Rotando..." : "Rotar token"}
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>
                              ¿Rotar el token de ingesta?
                            </AlertDialogTitle>
                            <AlertDialogDescription>
                              El token actual deja de funcionar de inmediato. El
                              sistema externo que envía datos a este monitor
                              necesitará el nuevo token para seguir funcionando.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancelar</AlertDialogCancel>
                            <AlertDialogAction onClick={rotateToken}>
                              Rotar
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  )
                ) : (
                  <>
                    <div
                      {...getRootProps()}
                      className={`flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-12 transition-colors ${
                        isDragActive
                          ? "border-primary bg-primary/5"
                          : "border-muted-foreground/25 hover:border-primary/50"
                      }`}
                    >
                      <input {...getInputProps()} />
                      <Upload className="mb-4 h-10 w-10 text-muted-foreground" />
                      {uploading || checking ? (
                        <p className="text-sm text-muted-foreground">
                          {checking ? "Verificando archivo..." : "Procesando archivo..."}
                        </p>
                      ) : isDragActive ? (
                        <p className="text-sm">Suelta el archivo aqui</p>
                      ) : (
                        <>
                          <p className="text-sm font-medium">
                            Arrastra un archivo o haz clic para seleccionar
                          </p>
                          <p className="mt-1 text-xs text-muted-foreground">
                            CSV, Excel (.xlsx), JSON, TXT
                          </p>
                        </>
                      )}
                    </div>

                    {uploadResult && (
                      <div className="mt-4 rounded-md bg-success-bg p-4">
                        <p className="text-sm font-medium text-success-fg">
                          {uploadResult.recordsIngested} registros cargados
                          {uploadResult.evaluationQueued &&
                            " — reglas en evaluacion"}
                        </p>
                      </div>
                    )}
                  </>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="data">
            <Card>
              <CardContent className="pt-6">
                {data.length === 0 ? (
                  <p className="text-center text-sm text-muted-foreground">
                    No hay datos cargados
                  </p>
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
                          {
                            data.filter((row) =>
                              Object.values(row).some((v) =>
                                String(v ?? "")
                                  .toLowerCase()
                                  .includes(searchTerm.toLowerCase()),
                              ),
                            ).length
                          }{" "}
                          resultados
                        </span>
                      )}
                    </div>
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b">
                            {monitor.schema?.map((field: SchemaField) => (
                              <th
                                key={field.name}
                                className="px-3 py-2 text-left font-medium text-muted-foreground"
                              >
                                {field.name}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {data
                            .filter(
                              (row) =>
                                !searchTerm ||
                                Object.values(row).some((v) =>
                                  String(v ?? "")
                                    .toLowerCase()
                                    .includes(searchTerm.toLowerCase()),
                                ),
                            )
                            .slice(0, 100)
                            .map((row, i) => (
                              <tr
                                key={i}
                                className="border-b last:border-0 hover:bg-muted/50"
                              >
                                {monitor.schema?.map((field: SchemaField) => {
                                  const val = String(row[field.name] ?? "");
                                  const isMatch =
                                    searchTerm &&
                                    val
                                      .toLowerCase()
                                      .includes(searchTerm.toLowerCase());
                                  return (
                                    <td
                                      key={field.name}
                                      className={`max-w-[200px] truncate px-3 py-2 ${isMatch ? "bg-warning-bg font-medium" : ""}`}
                                    >
                                      {val}
                                    </td>
                                  );
                                })}
                              </tr>
                            ))}
                        </tbody>
                      </table>
                      <p className="mt-2 text-center text-xs text-muted-foreground">
                        Mostrando{" "}
                        {Math.min(
                          100,
                          data.filter(
                            (row) =>
                              !searchTerm ||
                              Object.values(row).some((v) =>
                                String(v ?? "")
                                  .toLowerCase()
                                  .includes(searchTerm.toLowerCase()),
                              ),
                          ).length,
                        )}{" "}
                        de {monitor.recordCount} registros
                      </p>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="schema">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Esquema detectado</CardTitle>
                {monitor.schema && monitor.schema.length > 0 && user?.role !== "viewer" && (
                  <Button size="sm" onClick={handleSaveSchema} disabled={savingSchema}>
                    {savingSchema ? "Guardando..." : "Guardar"}
                  </Button>
                )}
              </CardHeader>
              <CardContent>
                {!monitor.schema || monitor.schema.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    Carga datos para detectar el esquema automaticamente
                  </p>
                ) : (
                  <div className="space-y-2">
                    {schemaEdits.map((field: SchemaField) => (
                      <div
                        key={field.name}
                        className="flex items-center justify-between rounded-md border p-3"
                      >
                        <div>
                          <p className="text-sm font-medium">{field.name}</p>
                          {field.sample && (
                            <p className="text-xs text-muted-foreground">
                              Ejemplo: {field.sample}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-2">
                          {field.type === "number" && user?.role !== "viewer" && (
                            <Select
                              value={String(field.impliedDecimals ?? 0)}
                              onValueChange={(v) => updateImpliedDecimals(field.name, Number(v))}
                            >
                              <SelectTrigger className="w-[180px]">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="0">Sin decimales implícitos</SelectItem>
                                <SelectItem value="1">1 decimal implícito</SelectItem>
                                <SelectItem value="2">2 decimales implícitos</SelectItem>
                                <SelectItem value="3">3 decimales implícitos</SelectItem>
                                <SelectItem value="4">4 decimales implícitos</SelectItem>
                              </SelectContent>
                            </Select>
                          )}
                          {field.type === "date" &&
                            // El campo derivado ya es una fecha real: nunca se
                            // parsea desde texto, así que ofrecerle un formato
                            // de lectura solo confunde.
                            field.name !== monitor.derivedTimestamp?.targetName &&
                            user?.role !== "viewer" &&
                            (monitor.sourceType === "csv" ||
                              monitor.sourceType === "txt" ||
                              monitor.sourceType === "excel") && (
                              <Select
                                value={field.dateFormat || "auto"}
                                onValueChange={(v) =>
                                  updateDateFormat(field.name, v === "auto" ? "" : v)
                                }
                              >
                                <SelectTrigger className="w-[180px]">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectItem value="auto">Automático (ISO / RFC3339)</SelectItem>
                                  <SelectItem value="DD/MM/YYYY">DD/MM/YYYY</SelectItem>
                                  <SelectItem value="MM/DD/YYYY">MM/DD/YYYY</SelectItem>
                                  <SelectItem value="DD-MM-YYYY">DD-MM-YYYY</SelectItem>
                                  <SelectItem value="YYYY/MM/DD">YYYY/MM/DD</SelectItem>
                                </SelectContent>
                              </Select>
                            )}
                          <Badge variant="secondary">{field.type}</Badge>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>

            {monitor.schema && monitor.schema.length > 0 && (
              <div className="mt-4">
                <DerivedTimestampCard
                  monitor={monitor}
                  canEdit={user?.role !== "viewer"}
                  onSaved={(actualizado) => setMonitor(actualizado)}
                />
              </div>
            )}
          </TabsContent>

          <TabsContent value="rules">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between">
                <CardTitle className="text-base">Reglas del monitor</CardTitle>
                <Button size="sm" asChild>
                  <Link href={`/rules?monitorId=${id}`}>
                    <ShieldCheck className="mr-2 h-4 w-4" />
                    Gestionar reglas
                  </Link>
                </Button>
              </CardHeader>
              <CardContent>
                {rules.length === 0 ? (
                  <p className="text-sm text-muted-foreground">
                    No hay reglas configuradas para este monitor
                  </p>
                ) : (
                  <div className="space-y-2">
                    {rules.map((rule) => (
                      <div
                        key={rule.id}
                        className="flex items-center justify-between rounded-md border p-3"
                      >
                        <div>
                          <p className="text-sm font-medium">{rule.name}</p>
                          <p className="text-xs text-muted-foreground">
                            {rule.description}
                          </p>
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge
                            variant={rule.active ? "success" : "secondary"}
                          >
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

      <AlertDialog open={pendingFile !== null} onOpenChange={(open) => !open && setPendingFile(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>El archivo no coincide con la estructura esperada</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2 text-sm">
                {checkResult?.missingFields && checkResult.missingFields.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Faltan columnas: </span>
                    {checkResult.missingFields.join(", ")}
                  </p>
                )}
                {checkResult?.extraFields && checkResult.extraFields.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Columnas de más: </span>
                    {checkResult.extraFields.join(", ")}
                  </p>
                )}
                {checkResult?.typeMismatches && checkResult.typeMismatches.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Tipo distinto: </span>
                    {checkResult.typeMismatches
                      .map((m) => `${m.field} (esperado ${m.expectedType}, encontrado ${m.actualType})`)
                      .join(", ")}
                  </p>
                )}
                {checkResult && checkResult.rowsThatWouldFail > 0 && (
                  <p>
                    {checkResult.rowsThatWouldFail} de {checkResult.totalRows} filas fallarían por tipo
                    de dato incorrecto.
                  </p>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingFile(null)}>Cancelar</AlertDialogCancel>
            <AlertDialogAction onClick={confirmUploadAnyway}>Subir de todas formas</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
