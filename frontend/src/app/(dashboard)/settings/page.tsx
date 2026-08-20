"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Settings, Server, Database, Brain, CheckCircle, XCircle, Clock, Save, Loader2 } from "lucide-react";
import { api } from "@/lib/api/client";
import { settingsApi, type AIConfigResponse } from "@/lib/api/settings";
import { useToast } from "@/lib/use-toast";

interface SystemSettings {
  port: string;
  mongoConnected: boolean;
  redisConnected: boolean;
  aiEnabled: boolean;
  aiProvider: string;
  aiModel: string;
  allowedOrigins: string;
  workers: number;
  maxRetries: number;
  scheduler?: {
    running: boolean;
    rulesActive: number;
  };
}

const providerLabels: Record<string, string> = {
  anthropic: "Anthropic (Claude)",
  deepseek: "DeepSeek",
};

function StatusDot({ ok }: { ok: boolean }) {
  return ok ? (
    <CheckCircle className="h-4 w-4 text-emerald-500" />
  ) : (
    <XCircle className="h-4 w-4 text-red-500" />
  );
}

function InfoRow({ label, value, status }: { label: string; value: string; status?: boolean }) {
  return (
    <div className="flex items-center justify-between rounded-md border p-3">
      <span className="text-sm text-muted-foreground">{label}</span>
      <div className="flex items-center gap-2">
        {status !== undefined && <StatusDot ok={status} />}
        <span className="text-sm font-medium">{value}</span>
      </div>
    </div>
  );
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<SystemSettings | null>(null);
  const [error, setError] = useState("");
  const { toastSuccess, toastError } = useToast();

  // AI config form state
  const [aiConfig, setAiConfig] = useState<AIConfigResponse | null>(null);
  const [aiProvider, setAiProvider] = useState("anthropic");
  const [aiModel, setAiModel] = useState("");
  const [aiApiKey, setAiApiKey] = useState("");
  const [aiSaving, setAiSaving] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const [sys, ai] = await Promise.all([
          api.get<SystemSettings>("/settings"),
          settingsApi.getAIConfig(),
        ]);
        setSettings(sys);
        setAiConfig(ai);
        setAiProvider(ai.provider);
        setAiModel(ai.model);
      } catch {
        setError("No se pudo cargar la configuracion");
      }
    }
    load();
  }, []);

  async function saveAIConfig() {
    setAiSaving(true);
    try {
      await settingsApi.updateAIConfig({
        provider: aiProvider,
        model: aiModel,
        ...(aiApiKey ? { apiKey: aiApiKey } : {}),
      });
      toastSuccess("Configuracion de IA actualizada");
      // Refresh AI config to get updated masked key
      const fresh = await settingsApi.getAIConfig();
      setAiConfig(fresh);
      setAiApiKey("");

      // Refresh system settings too
      const sys = await api.get<SystemSettings>("/settings");
      setSettings(sys);
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al guardar configuracion");
    } finally {
      setAiSaving(false);
    }
  }

  // Models available for current provider
  const availableModels = aiConfig?.availableModels?.[aiProvider] ?? [];

  // When provider changes, auto-select first model
  function handleProviderChange(provider: string) {
    setAiProvider(provider);
    const models = aiConfig?.availableModels?.[provider] ?? [];
    if (models.length > 0) {
      setAiModel(models[0]);
    }
  }

  if (error) {
    return (
      <>
        <Header title="Configuracion" />
        <div className="p-6">
          <p className="text-sm text-red-500">{error}</p>
        </div>
      </>
    );
  }

  if (!settings) {
    return (
      <>
        <Header title="Configuracion" />
        <div className="p-6">
          <p className="text-sm text-muted-foreground">Cargando...</p>
        </div>
      </>
    );
  }

  return (
    <>
      <Header title="Configuracion" />
      <div className="p-6">
        <div className="mb-6">
          <p className="text-sm text-muted-foreground">
            Configuracion del sistema y servicios
          </p>
        </div>

        <div className="space-y-6">
          {/* AI Configuration — EDITABLE */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Brain className="h-4 w-4" /> Inteligencia Artificial
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="ai-provider">Proveedor</Label>
                  <Select value={aiProvider} onValueChange={handleProviderChange}>
                    <SelectTrigger id="ai-provider">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="anthropic">Anthropic (Claude)</SelectItem>
                      <SelectItem value="deepseek">DeepSeek</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="ai-model">Modelo</Label>
                  <Select value={aiModel} onValueChange={setAiModel}>
                    <SelectTrigger id="ai-model">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {availableModels.map((m) => (
                        <SelectItem key={m} value={m}>{m}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="ai-key">API Key</Label>
                <div className="flex gap-2">
                  <Input
                    id="ai-key"
                    type="password"
                    placeholder={aiConfig?.apiKeySet ? aiConfig.apiKeyMasked : "Ingresa tu API key"}
                    value={aiApiKey}
                    onChange={(e) => setAiApiKey(e.target.value)}
                  />
                </div>
                {aiConfig?.apiKeySet && !aiApiKey && (
                  <p className="text-xs text-muted-foreground">
                    Key actual: {aiConfig.apiKeyMasked} — deja vacio para mantenerla
                  </p>
                )}
              </div>

              <div className="flex items-center justify-between pt-2">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <StatusDot ok={aiConfig?.apiKeySet ?? false} />
                  {aiConfig?.apiKeySet ? "API Key configurada" : "API Key no configurada"}
                </div>
                <Button onClick={saveAIConfig} disabled={aiSaving} size="sm">
                  {aiSaving ? (
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  ) : (
                    <Save className="mr-2 h-4 w-4" />
                  )}
                  Guardar
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Server — read-only */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Server className="h-4 w-4" /> Servidor
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <InfoRow label="Puerto" value={settings.port} />
              <InfoRow label="Workers de evaluacion" value={String(settings.workers)} />
              <InfoRow label="Max reintentos (dead queue)" value={String(settings.maxRetries)} />
              <InfoRow label="Origenes permitidos" value={settings.allowedOrigins} />
            </CardContent>
          </Card>

          {/* Database — read-only */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Database className="h-4 w-4" /> Base de datos
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <InfoRow label="MongoDB" value={settings.mongoConnected ? "Conectado" : "Desconectado"} status={settings.mongoConnected} />
              <InfoRow label="Redis" value={settings.redisConnected ? "Conectado" : "Desconectado"} status={settings.redisConnected} />
            </CardContent>
          </Card>

          {/* Scheduler — read-only */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Clock className="h-4 w-4" /> Programador de reglas
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <InfoRow
                label="Estado"
                value={settings.scheduler?.running ? "Activo" : "Inactivo"}
                status={settings.scheduler?.running ?? false}
              />
              <InfoRow
                label="Reglas programadas"
                value={String(settings.scheduler?.rulesActive ?? 0)}
              />
            </CardContent>
          </Card>

          {/* Frontend — read-only */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Settings className="h-4 w-4" /> Frontend
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <InfoRow label="API URL" value={process.env.NEXT_PUBLIC_API_URL || "No configurada"} />
              <InfoRow label="Version" value="1.0.0" />
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
