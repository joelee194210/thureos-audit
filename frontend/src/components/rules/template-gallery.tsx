"use client";

import { useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { useToast } from "@/lib/use-toast";
import { rulesApi, type RuleTemplate } from "@/lib/api/rules";
import type { Monitor } from "@/lib/types";
import { RISK_CLASSES } from "@/lib/semantic-colors";

interface TemplateGalleryProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  monitors: Monitor[];
  defaultMonitorId?: string;
  onCreated: () => void;
}

export function TemplateGallery({
  open,
  onOpenChange,
  monitors,
  defaultMonitorId,
  onCreated,
}: TemplateGalleryProps) {
  const { toastError, toastSuccess } = useToast();
  const [templates, setTemplates] = useState<RuleTemplate[]>([]);
  const [selected, setSelected] = useState<RuleTemplate | null>(null);
  const [monitorId, setMonitorId] = useState(defaultMonitorId || "");
  const [fieldMap, setFieldMap] = useState<Record<string, string>>({});
  const [params, setParams] = useState<Record<string, string>>({});
  const [ruleName, setRuleName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    rulesApi
      .listTemplates()
      .then(setTemplates)
      .catch(() => toastError("Error al cargar tipologías"));
    return () => setSelected(null);
  }, [open]);

  const monitor = monitors.find((m) => m.id === monitorId);

  function chooseTemplate(t: RuleTemplate) {
    setSelected(t);
    setFieldMap({});
    setParams(
      Object.fromEntries(t.params.map((p) => [p.key, String(p.default)])),
    );
    setRuleName(t.name);
  }

  const allFieldsMapped =
    selected != null &&
    selected.requiredFields.every((f) => (fieldMap[f] ?? "").trim() !== "");

  async function instantiate() {
    if (!selected || !monitorId || !allFieldsMapped || !ruleName.trim()) return;
    setSubmitting(true);
    try {
      const numericParams = Object.fromEntries(
        Object.entries(params)
          .map(([k, v]) => [k, Number(v)])
          .filter(([, v]) => Number.isFinite(v)),
      );
      await rulesApi.instantiateTemplate(monitorId, {
        templateId: selected.id,
        fieldMap,
        params: numericParams,
        name: ruleName.trim(),
      });
      toastSuccess("Regla creada desde tipología");
      setSelected(null);
      onOpenChange(false);
      onCreated();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al instanciar");
    } finally {
      setSubmitting(false);
    }
  }

  const severityClass = (severity: string) =>
    RISK_CLASSES[
      severity === "critical" || severity === "high" ? "high" : "medium"
    ] ?? "";

  if (!selected) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>Tipologías AML</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {templates.map((t) => (
              <button
                key={t.id}
                onClick={() => chooseTemplate(t)}
                className="text-left rounded-lg border p-4 hover:bg-accent/50 transition-colors"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-semibold">{t.name}</span>
                  <span
                    className={`text-[10px] uppercase font-semibold px-2 py-0.5 rounded ${severityClass(t.suggestedSeverity)}`}
                  >
                    {t.suggestedSeverity}
                  </span>
                </div>
                <p className="mt-1 text-xs text-ink-muted">{t.description}</p>
              </button>
            ))}
            {templates.length === 0 && (
              <p className="text-sm text-ink-muted col-span-2">Cargando tipologías…</p>
            )}
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  // --- Formulario de instanciación ---
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{selected.name}</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-ink-muted -mt-2">{selected.description}</p>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Monitor</Label>
            <Select value={monitorId} onValueChange={setMonitorId}>
              <SelectTrigger>
                <SelectValue placeholder="Elegí un monitor" />
              </SelectTrigger>
              <SelectContent>
                {monitors.map((m) => (
                  <SelectItem key={m.id} value={m.id}>
                    {m.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>Nombre de la regla</Label>
            <Input
              value={ruleName}
              onChange={(e) => setRuleName(e.target.value)}
              placeholder="Nombre con el que aparecerá en la lista"
            />
          </div>

          <div className="space-y-3">
            <Label>Campos del esquema</Label>
            {selected.requiredFields.map((f) => (
              <div key={f} className="grid grid-cols-[110px_1fr] items-center gap-2">
                <span className="font-mono text-xs">{f}</span>
                <Select
                  value={fieldMap[f] ?? ""}
                  onValueChange={(v) =>
                    setFieldMap((prev) => ({ ...prev, [f]: v }))
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Elegí una columna" />
                  </SelectTrigger>
                  <SelectContent>
                    {(monitor?.schema ?? []).map((sf) => (
                      <SelectItem key={sf.name} value={sf.name}>
                        {sf.name}{" "}
                        <span className="text-xs text-ink-muted">
                          ({sf.type})
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            ))}
          </div>

          {selected.params.length > 0 && (
            <div className="space-y-3">
              <Label>Parámetros</Label>
              {selected.params.map((p) => (
                <div key={p.key} className="grid grid-cols-[110px_1fr] items-center gap-2">
                  <span className="text-xs">{p.label}</span>
                  <Input
                    type="number"
                    value={params[p.key] ?? ""}
                    onChange={(e) =>
                      setParams((prev) => ({ ...prev, [p.key]: e.target.value }))
                    }
                  />
                </div>
              ))}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setSelected(null)}>
              Volver
            </Button>
            <Button
              onClick={instantiate}
              disabled={
                submitting || !monitorId || !allFieldsMapped || !ruleName.trim()
              }
            >
              {submitting ? "Creando…" : "Crear regla"}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
