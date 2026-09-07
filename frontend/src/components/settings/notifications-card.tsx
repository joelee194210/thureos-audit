"use client";

import { useEffect, useState } from "react";
import { Bell } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/lib/use-toast";
import {
  settingsApi,
  type NotificationSettings,
  type NotificationSettingsUpdate,
} from "@/lib/api/settings";

interface WebhookForm {
  url: string;
  secret: string;
  enabled: boolean;
  secretSet?: boolean;
}

const EMPTY: NotificationSettings = {
  emailProvider: "smtp",
  smtp: { host: "", port: 587, username: "", from: "", passwordSet: false },
  resend: { from: "", apiKeySet: false },
  toEmails: [],
  webhooks: [],
};

export function NotificationsCard() {
  const { toastError, toastSuccess } = useToast();
  const [cfg, setCfg] = useState<NotificationSettings>(EMPTY);
  const [smtpPassword, setSmtpPassword] = useState("");
  const [resendKey, setResendKey] = useState("");
  const [webhooks, setWebhooks] = useState<WebhookForm[]>([]);
  const [toEmails, setToEmails] = useState("");
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  useEffect(() => {
    settingsApi
      .getNotifications()
      .then((data) => {
        setCfg({ ...EMPTY, ...data });
        setToEmails((data.toEmails ?? []).join(", "));
        setWebhooks(
          (data.webhooks ?? []).map((w) => ({
            url: w.url,
            secret: "",
            enabled: w.enabled,
            secretSet: w.secretSet,
          })),
        );
      })
      .catch(() => toastError("Error al cargar notificaciones"));
  }, []);

  async function save() {
    setSaving(true);
    try {
      const body: NotificationSettingsUpdate = {
        emailProvider: cfg.emailProvider,
        smtp: {
          host: cfg.smtp.host,
          port: Number(cfg.smtp.port) || 587,
          username: cfg.smtp.username,
          from: cfg.smtp.from,
          ...(smtpPassword ? { password: smtpPassword } : {}),
        },
        resend: {
          from: cfg.resend.from,
          ...(resendKey ? { apiKey: resendKey } : {}),
        },
        toEmails: toEmails
          .split(",")
          .map((e) => e.trim())
          .filter(Boolean),
        webhooks: webhooks.map((w) => ({
          url: w.url,
          enabled: w.enabled,
          ...(w.secret ? { secret: w.secret } : {}),
        })),
      };
      await settingsApi.updateNotifications(body);
      toastSuccess("Notificaciones guardadas");
      setSmtpPassword("");
      setResendKey("");
      setWebhooks((prev) => prev.map((w) => ({ ...w, secret: "", secretSet: true })));
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al guardar");
    } finally {
      setSaving(false);
    }
  }

  async function test() {
    setTesting(true);
    try {
      await settingsApi.testNotifications();
      toastSuccess("Notificación de prueba encolada");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al probar");
    } finally {
      setTesting(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Bell className="h-4 w-4" /> Notificaciones de banderas rojas
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="space-y-2">
          <Label>Proveedor de email</Label>
          <div className="flex gap-2">
            {(["smtp", "resend"] as const).map((prov) => (
              <button
                key={prov}
                type="button"
                onClick={() =>
                  setCfg((prev) => ({ ...prev, emailProvider: prov }))
                }
                className={`rounded-md border px-4 py-2 text-sm transition-colors ${
                  cfg.emailProvider === prov
                    ? "border-primary bg-primary/10 font-medium"
                    : "hover:bg-accent/50"
                }`}
              >
                {prov === "smtp" ? "SMTP directo" : "API de Resend"}
              </button>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">
            Se usa para enviar el aviso de cada bandera roja nueva.
          </p>
        </div>

        {cfg.emailProvider === "smtp" ? (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>Host SMTP</Label>
              <Input
                value={cfg.smtp.host}
                onChange={(e) =>
                  setCfg((p) => ({ ...p, smtp: { ...p.smtp, host: e.target.value } }))
                }
                placeholder="smtp.gmail.com"
              />
            </div>
            <div className="space-y-1.5">
              <Label>Puerto</Label>
              <Input
                type="number"
                value={cfg.smtp.port}
                onChange={(e) =>
                  setCfg((p) => ({
                    ...p,
                    smtp: { ...p.smtp, port: Number(e.target.value) },
                  }))
                }
              />
            </div>
            <div className="space-y-1.5">
              <Label>Usuario</Label>
              <Input
                value={cfg.smtp.username}
                onChange={(e) =>
                  setCfg((p) => ({
                    ...p,
                    smtp: { ...p.smtp, username: e.target.value },
                  }))
                }
              />
            </div>
            <div className="space-y-1.5">
              <Label>Contraseña</Label>
              <Input
                type="password"
                value={smtpPassword}
                onChange={(e) => setSmtpPassword(e.target.value)}
                placeholder={
                  cfg.smtp.passwordSet ? "•••••••• (sin cambios)" : "Contraseña"
                }
              />
            </div>
            <div className="space-y-1.5 md:col-span-2">
              <Label>Remitente (From)</Label>
              <Input
                value={cfg.smtp.from}
                onChange={(e) =>
                  setCfg((p) => ({ ...p, smtp: { ...p.smtp, from: e.target.value } }))
                }
                placeholder="alertas@tuempresa.com"
              />
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label>API Key de Resend</Label>
              <Input
                type="password"
                value={resendKey}
                onChange={(e) => setResendKey(e.target.value)}
                placeholder={
                  cfg.resend.apiKeySet ? "•••••••• (sin cambios)" : "re_..."
                }
              />
            </div>
            <div className="space-y-1.5">
              <Label>Remitente (From)</Label>
              <Input
                value={cfg.resend.from}
                onChange={(e) =>
                  setCfg((p) => ({
                    ...p,
                    resend: { ...p.resend, from: e.target.value },
                  }))
                }
                placeholder="alertas@tuempresa.com"
              />
            </div>
          </div>
        )}

        <div className="space-y-1.5">
          <Label>Destinatarios (separados por coma)</Label>
          <Input
            value={toEmails}
            onChange={(e) => setToEmails(e.target.value)}
            placeholder="oficial@empresa.com, analista@empresa.com"
          />
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Webhooks</Label>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                setWebhooks((prev) => [
                  ...prev,
                  { url: "", secret: "", enabled: true },
                ])
              }
            >
              Agregar webhook
            </Button>
          </div>
          {webhooks.length === 0 && (
            <p className="text-xs text-muted-foreground">
              Sin webhooks. Cada bandera roja nueva llega firmada (HMAC
              SHA-256) en el header X-Thureos-Signature.
            </p>
          )}
          {webhooks.map((w, i) => (
            <div
              key={i}
              className="grid grid-cols-[1fr_140px_auto_auto] items-center gap-2 rounded-md border p-2"
            >
              <Input
                value={w.url}
                onChange={(e) =>
                  setWebhooks((prev) =>
                    prev.map((x, j) =>
                      j === i ? { ...x, url: e.target.value } : x,
                    ),
                  )
                }
                placeholder="https://…"
              />
              <Input
                value={w.secret}
                onChange={(e) =>
                  setWebhooks((prev) =>
                    prev.map((x, j) =>
                      j === i ? { ...x, secret: e.target.value } : x,
                    ),
                  )
                }
                placeholder={
                  w.secretSet ? "•••••••• (sin cambios)" : "Secreto (opcional)"
                }
              />
              <Switch
                checked={w.enabled}
                onCheckedChange={(v) =>
                  setWebhooks((prev) =>
                    prev.map((x, j) => (j === i ? { ...x, enabled: v } : x)),
                  )
                }
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={() =>
                  setWebhooks((prev) => prev.filter((_, j) => j !== i))
                }
              >
                Quitar
              </Button>
            </div>
          ))}
        </div>

        <div className="flex justify-end gap-2">
          <Button
            variant="outline"
            onClick={test}
            disabled={testing || saving}
          >
            {testing ? "Probando…" : "Probar"}
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving ? "Guardando…" : "Guardar notificaciones"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
