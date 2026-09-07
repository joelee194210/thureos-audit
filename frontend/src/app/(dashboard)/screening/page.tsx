"use client";

import { useState } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { useToast } from "@/lib/use-toast";
import { screeningApi, type ScreeningResult } from "@/lib/api/screening";

const STATUS_LABEL: Record<string, string> = {
  clear: "Limpio",
  match: "Coincidencia",
  review: "Revisar",
};

const STATUS_CLASS: Record<string, string> = {
  clear: "bg-success/15 text-success",
  match: "bg-danger/15 text-danger",
  review: "bg-warning/15 text-warning",
};

export default function ScreeningPage() {
  const { toastError } = useToast();
  const [name, setName] = useState("");
  const [dateOfBirth, setDateOfBirth] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ScreeningResult | null>(null);

  async function search(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setLoading(true);
    setResult(null);
    try {
      const res = await screeningApi.search(name, dateOfBirth || undefined);
      setResult(res);
    } catch (err) {
      toastError(
        err instanceof Error ? err.message : "Error al buscar en Watchman",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Header title="Screening de sanciones" />
      <div className="p-6 space-y-4 max-w-2xl">
        <p className="text-sm text-muted-foreground">
          Búsqueda ad-hoc contra OFAC/ONU/UE/UK (Watchman). No queda
          registrada — para screening con caso asociado, ver el detalle de
          la bandera roja correspondiente.
        </p>
        <form onSubmit={search} className="flex gap-2 items-end">
          <div className="flex-1 space-y-1.5">
            <Label>Nombre</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Nombre completo"
              required
            />
          </div>
          <div className="w-40 space-y-1.5">
            <Label>Fecha de nac. (opcional)</Label>
            <Input
              type="date"
              value={dateOfBirth}
              onChange={(e) => setDateOfBirth(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={loading}>
            {loading ? "Buscando…" : "Buscar"}
          </Button>
        </form>

        {result && (
          <Card>
            <CardContent className="pt-6 space-y-3">
              <div className="flex items-center gap-2">
                <Badge className={STATUS_CLASS[result.status] || ""}>
                  {STATUS_LABEL[result.status] || result.status}
                </Badge>
                <span className="text-sm text-muted-foreground">
                  {result.matches?.length || 0} coincidencia(s)
                </span>
              </div>
              {(result.matches || []).map((m, i) => (
                <div key={i} className="rounded-md border p-3 text-sm">
                  <p className="font-medium">{m.name}</p>
                  <p className="text-xs text-muted-foreground">
                    {m.sourceList} · score {m.score.toFixed(2)}
                    {m.programs && m.programs.length > 0
                      ? ` · ${m.programs.join(", ")}`
                      : ""}
                  </p>
                </div>
              ))}
            </CardContent>
          </Card>
        )}
      </div>
    </>
  );
}
