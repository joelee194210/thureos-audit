"use client";

import { useEffect, useState } from "react";
import { ScanSearch } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useToast } from "@/lib/use-toast";
import { screeningApi } from "@/lib/api/screening";

export function ScreeningCard() {
  const { toastError, toastSuccess } = useToast();
  const [watchmanUrl, setWatchmanUrl] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    screeningApi
      .getConfig()
      .then((cfg) => setWatchmanUrl(cfg.watchmanUrl))
      .catch(() => toastError("Error al cargar configuración de screening"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save() {
    setSaving(true);
    try {
      await screeningApi.updateConfig(watchmanUrl);
      toastSuccess("Configuración de screening guardada");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al guardar");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <ScanSearch className="h-4 w-4" /> Screening de sanciones
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-1.5">
          <Label>URL de Watchman</Label>
          <Input
            value={watchmanUrl}
            onChange={(e) => setWatchmanUrl(e.target.value)}
            placeholder="http://marion-watchman:8084"
          />
          <p className="text-xs text-muted-foreground">
            Servicio de screening contra OFAC/ONU/UE/UK — se opera fuera de
            esta app; acá solo se configura la URL.
          </p>
        </div>
        <div className="flex justify-end">
          <Button onClick={save} disabled={saving}>
            {saving ? "Guardando…" : "Guardar"}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
