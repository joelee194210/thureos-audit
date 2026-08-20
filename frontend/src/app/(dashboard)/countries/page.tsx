"use client";

import { useEffect, useState, useMemo } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Search,
  Globe,
  ShieldAlert,
  Shield,
  ShieldCheck,
  Pencil,
  Check,
  X,
  Plus,
} from "lucide-react";
import { countriesApi } from "@/lib/api/countries";
import { cn } from "@/lib/utils";
import { CATEGORY_CLASSES, RISK_BG, RISK_CLASSES, RISK_FG, STATUS_CLASSES } from "@/lib/semantic-colors";
import type { Country, CountryRiskLevel } from "@/lib/types";

const riskConfig: Record<
  string,
  { label: string; color: string; icon: typeof ShieldAlert }
> = {
  high: {
    label: "Alto",
    color:
      `${RISK_CLASSES.high} border-transparent`,
    icon: ShieldAlert,
  },
  medium: {
    label: "Medio",
    color:
      `${RISK_CLASSES.medium} border-transparent`,
    icon: Shield,
  },
  low: {
    label: "Bajo",
    color:
      `${RISK_CLASSES.low} border-transparent`,
    icon: ShieldCheck,
  },
};

const sourceColors: Record<string, string> = {
  FATF: CATEGORY_CLASSES[5],
  OFAC: CATEGORY_CLASSES[0],
  ONU: CATEGORY_CLASSES[4],
  EU: CATEGORY_CLASSES[3],
  Basel: CATEGORY_CLASSES[6],
};

export default function CountriesPage() {
  const [countries, setCountries] = useState<Country[]>([]);
  const [regions, setRegions] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [selectedRegion, setSelectedRegion] = useState("");
  const [selectedRisk, setSelectedRisk] = useState("");
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editForm, setEditForm] = useState<Partial<Country>>({});

  async function load() {
    try {
      const [all, regs] = await Promise.all([
        countriesApi.list(),
        countriesApi.regions(),
      ]);
      setCountries(all);
      setRegions(regs);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    return countries.filter((c) => {
      if (selectedRegion && c.region !== selectedRegion) return false;
      if (selectedRisk && c.riskLevel !== selectedRisk) return false;
      if (search) {
        const q = search.toLowerCase();
        return (
          c.code.toLowerCase().includes(q) ||
          c.code3.toLowerCase().includes(q) ||
          c.name.toLowerCase().includes(q) ||
          c.nameEn.toLowerCase().includes(q)
        );
      }
      return true;
    });
  }, [countries, search, selectedRegion, selectedRisk]);

  const stats = useMemo(
    () => ({
      total: countries.length,
      high: countries.filter((c) => c.riskLevel === "high" && c.active).length,
      medium: countries.filter((c) => c.riskLevel === "medium" && c.active)
        .length,
      low: countries.filter((c) => c.riskLevel === "low" && c.active).length,
      regions: regions.length,
    }),
    [countries, regions]
  );

  // Group by region
  const grouped = useMemo(() => {
    const map = new Map<string, Country[]>();
    for (const c of filtered) {
      if (!map.has(c.region)) map.set(c.region, []);
      map.get(c.region)!.push(c);
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [filtered]);

  function startEdit(country: Country) {
    setEditingId(country.id);
    setEditForm({
      riskLevel: country.riskLevel,
      riskSources: country.riskSources,
      active: country.active,
      notes: country.notes,
    });
  }

  async function saveEdit(id: string) {
    try {
      await countriesApi.update(id, editForm);
      setEditingId(null);
      load();
    } catch {
      /* ignore */
    }
  }

  return (
    <>
      <Header title="Países y Riesgo" />
      <div className="p-6 space-y-6">
        {/* Stats */}
        <div className="grid grid-cols-5 gap-3">
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-primary/10 p-2">
                <Globe className="h-5 w-5 text-primary" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.total}</p>
                <p className="text-xs text-muted-foreground">Total países</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", RISK_BG.high)}>
                <ShieldAlert className={cn("h-5 w-5", RISK_FG.high)} />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.high}</p>
                <p className="text-xs text-muted-foreground">Riesgo alto</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", RISK_BG.medium)}>
                <Shield className={cn("h-5 w-5", RISK_FG.medium)} />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.medium}</p>
                <p className="text-xs text-muted-foreground">Riesgo medio</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className={cn("rounded-lg p-2", RISK_BG.low)}>
                <ShieldCheck className={cn("h-5 w-5", RISK_FG.low)} />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.low}</p>
                <p className="text-xs text-muted-foreground">Riesgo bajo</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-accent-soft p-2">
                <Globe className="h-5 w-5 text-accent-fg" />
              </div>
              <div>
                <p className="text-2xl font-bold">{stats.regions}</p>
                <p className="text-xs text-muted-foreground">Regiones</p>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Filters */}
        <div className="space-y-3">
          <div className="flex gap-3">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-9"
                placeholder="Buscar por código, nombre o país..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="flex gap-1">
              {(["high", "medium", "low"] as const).map((level) => {
                const cfg = riskConfig[level];
                return (
                  <button
                    key={level}
                    onClick={() =>
                      setSelectedRisk(selectedRisk === level ? "" : level)
                    }
                    className={cn(
                      "rounded-md border px-3 py-1.5 text-xs font-medium transition-colors",
                      selectedRisk === level ? cfg.color : "hover:bg-accent"
                    )}
                  >
                    {cfg.label}
                  </button>
                );
              })}
            </div>
          </div>
          <div className="flex gap-1 flex-wrap">
            <button
              onClick={() => setSelectedRegion("")}
              className={cn(
                "rounded-md border px-2.5 py-1 text-xs transition-colors",
                !selectedRegion
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-accent"
              )}
            >
              Todas ({stats.total})
            </button>
            {regions.map((r) => {
              const count = countries.filter((c) => c.region === r).length;
              return (
                <button
                  key={r}
                  onClick={() =>
                    setSelectedRegion(selectedRegion === r ? "" : r)
                  }
                  className={cn(
                    "rounded-md border px-2.5 py-1 text-xs transition-colors",
                    selectedRegion === r
                      ? "bg-primary text-primary-foreground"
                      : "hover:bg-accent"
                  )}
                >
                  {r} ({count})
                </button>
              );
            })}
          </div>
        </div>

        {/* Results */}
        {loading ? (
          <div className="py-12 text-center text-muted-foreground">
            Cargando catalogo de países...
          </div>
        ) : (
          <div className="space-y-6">
            <p className="text-sm text-muted-foreground">
              Mostrando {filtered.length} de {countries.length} países
              {selectedRegion && (
                <>
                  {" "}
                  en <strong>{selectedRegion}</strong>
                </>
              )}
              {selectedRisk && (
                <>
                  {" "}
                  con riesgo{" "}
                  <strong>{riskConfig[selectedRisk]?.label}</strong>
                </>
              )}
            </p>

            {grouped.map(([region, items]) => (
              <div key={region}>
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="text-sm font-semibold">{region}</h3>
                  <Badge variant="secondary" className="text-[10px]">
                    {items.length} países
                  </Badge>
                </div>
                <div className="rounded-lg border overflow-hidden">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b bg-muted/50 text-xs text-muted-foreground">
                        <th className="px-4 py-2 text-left w-14">Código</th>
                        <th className="px-4 py-2 text-left">País</th>
                        <th className="px-4 py-2 text-left w-20">Riesgo</th>
                        <th className="px-4 py-2 text-left w-[180px]">
                          Fuentes
                        </th>
                        <th className="px-4 py-2 text-left">Notas</th>
                        <th className="px-4 py-2 text-center w-16">Estado</th>
                        <th className="px-4 py-2 text-center w-10"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {items.map((country) => {
                        const risk = riskConfig[country.riskLevel];
                        const isEditing = editingId === country.id;

                        if (isEditing) {
                          return (
                            <tr
                              key={country.id}
                              className="border-b last:border-0 bg-accent/30"
                            >
                              <td className="px-4 py-2 font-mono text-xs font-medium">
                                {country.code}
                              </td>
                              <td className="px-4 py-2">{country.name}</td>
                              <td className="px-4 py-2">
                                <Select
                                  value={editForm.riskLevel}
                                  onValueChange={(v) =>
                                    setEditForm({
                                      ...editForm,
                                      riskLevel: v as CountryRiskLevel,
                                    })
                                  }
                                >
                                  <SelectTrigger className="h-7 text-xs w-24">
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="high">Alto</SelectItem>
                                    <SelectItem value="medium">
                                      Medio
                                    </SelectItem>
                                    <SelectItem value="low">Bajo</SelectItem>
                                  </SelectContent>
                                </Select>
                              </td>
                              <td className="px-4 py-2">
                                <Input
                                  className="h-7 text-xs"
                                  value={editForm.riskSources?.join(", ") ?? ""}
                                  onChange={(e) =>
                                    setEditForm({
                                      ...editForm,
                                      riskSources: e.target.value
                                        .split(",")
                                        .map((s) => s.trim())
                                        .filter(Boolean),
                                    })
                                  }
                                  placeholder="FATF, OFAC, Basel..."
                                />
                              </td>
                              <td className="px-4 py-2">
                                <Input
                                  className="h-7 text-xs"
                                  value={editForm.notes ?? ""}
                                  onChange={(e) =>
                                    setEditForm({
                                      ...editForm,
                                      notes: e.target.value,
                                    })
                                  }
                                  placeholder="Notas..."
                                />
                              </td>
                              <td className="px-4 py-2 text-center">
                                <button
                                  onClick={() =>
                                    setEditForm({
                                      ...editForm,
                                      active: !editForm.active,
                                    })
                                  }
                                  className={cn(
                                    "rounded-full px-2 py-0.5 text-[10px] font-medium border",
                                    editForm.active
                                      ? STATUS_CLASSES.success
                                      : STATUS_CLASSES.neutral
                                  )}
                                >
                                  {editForm.active ? "Activo" : "Inactivo"}
                                </button>
                              </td>
                              <td className="px-4 py-2 text-center">
                                <div className="flex gap-1">
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-6 w-6"
                                    onClick={() => saveEdit(country.id)}
                                  >
                                    <Check className="h-3.5 w-3.5 text-success-fg" />
                                  </Button>
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-6 w-6"
                                    onClick={() => setEditingId(null)}
                                  >
                                    <X className="h-3.5 w-3.5 text-danger-fg" />
                                  </Button>
                                </div>
                              </td>
                            </tr>
                          );
                        }

                        return (
                          <tr
                            key={country.id}
                            className={cn(
                              "border-b last:border-0 hover:bg-muted/30 transition-colors",
                              !country.active && "opacity-50"
                            )}
                          >
                            <td className="px-4 py-2 font-mono text-xs font-medium">
                              {country.code}
                            </td>
                            <td className="px-4 py-2">
                              <div>
                                <span className="font-medium">
                                  {country.name}
                                </span>
                                <span className="ml-2 text-[10px] text-muted-foreground">
                                  {country.nameEn}
                                </span>
                              </div>
                            </td>
                            <td className="px-4 py-2">
                              <span
                                className={cn(
                                  "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium",
                                  risk?.color
                                )}
                              >
                                {risk?.label}
                              </span>
                            </td>
                            <td className="px-4 py-2">
                              <div className="flex gap-0.5 flex-wrap">
                                {country.riskSources.map((src) => (
                                  <span
                                    key={src}
                                    className={cn(
                                      "inline-block rounded px-1.5 py-0.5 text-[8px] font-bold leading-none whitespace-nowrap",
                                      sourceColors[src] ??
                                        "bg-surface-2 text-ink-muted"
                                    )}
                                  >
                                    {src}
                                  </span>
                                ))}
                                {country.riskSources.length === 0 && (
                                  <span className="text-[10px] text-muted-foreground">
                                    —
                                  </span>
                                )}
                              </div>
                            </td>
                            <td className="px-4 py-2 text-xs text-muted-foreground max-w-[200px] truncate">
                              {country.notes || "—"}
                            </td>
                            <td className="px-4 py-2 text-center">
                              <span
                                className={cn(
                                  "rounded-full px-2 py-0.5 text-[10px] font-medium border",
                                  country.active
                                    ? STATUS_CLASSES.success
                                    : STATUS_CLASSES.neutral
                                )}
                              >
                                {country.active ? "Activo" : "Inactivo"}
                              </span>
                            </td>
                            <td className="px-4 py-2 text-center">
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-6 w-6"
                                onClick={() => startEdit(country)}
                              >
                                <Pencil className="h-3 w-3 text-muted-foreground" />
                              </Button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            ))}

            {filtered.length === 0 && (
              <div className="py-12 text-center">
                <Globe className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
                <p className="font-medium">Sin resultados</p>
                <p className="text-sm text-muted-foreground">
                  Intenta con otra busqueda o filtro
                </p>
              </div>
            )}
          </div>
        )}
      </div>
    </>
  );
}
