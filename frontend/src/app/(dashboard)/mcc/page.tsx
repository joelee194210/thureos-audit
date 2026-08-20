"use client";

import { useEffect, useState, useMemo } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Search, CreditCard, ShieldAlert, ShieldCheck, Shield, Pencil } from "lucide-react";
import { mccApi } from "@/lib/api/mcc";
import { useToast } from "@/lib/use-toast";
import { cn } from "@/lib/utils";
import type { MCC, MCCRiskLevel, MCCNetwork } from "@/lib/types";

const networkLabels: Record<string, { label: string; short: string; color: string }> = {
  visa: { label: "Visa", short: "V", color: "bg-blue-600 text-white" },
  mastercard: { label: "Mastercard", short: "MC", color: "bg-orange-600 text-white" },
  unionpay: { label: "UnionPay", short: "UP", color: "bg-red-600 text-white" },
  amex: { label: "American Express", short: "AX", color: "bg-emerald-600 text-white" },
  discover: { label: "Discover", short: "DI", color: "bg-amber-600 text-white" },
  diners: { label: "Diners Club", short: "DC", color: "bg-indigo-600 text-white" },
  jcb: { label: "JCB", short: "JCB", color: "bg-teal-600 text-white" },
};

const riskConfig: Record<string, { label: string; color: string; icon: typeof ShieldAlert }> = {
  high: { label: "Alto", color: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 border-red-200 dark:border-red-800", icon: ShieldAlert },
  medium: { label: "Medio", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400 border-yellow-200 dark:border-yellow-800", icon: Shield },
  low: { label: "Bajo", color: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400 border-green-200 dark:border-green-800", icon: ShieldCheck },
};

const ALL_NETWORKS: MCCNetwork[] = ["visa", "mastercard", "unionpay", "amex", "discover", "diners", "jcb"];

export default function MCCPage() {
  const [mccs, setMccs] = useState<MCC[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [selectedCat, setSelectedCat] = useState("");
  const [selectedRisk, setSelectedRisk] = useState("");
  const [loading, setLoading] = useState(true);
  const [editingMcc, setEditingMcc] = useState<MCC | null>(null);
  const [editForm, setEditForm] = useState({ riskLevel: "low" as MCCRiskLevel, networks: [] as MCCNetwork[], description: "" });
  const { toastError, toastSuccess } = useToast();

  function loadMccs() {
    Promise.all([mccApi.list(), mccApi.categories()])
      .then(([all, cats]) => { setMccs(all); setCategories(cats); })
      .catch((err) => { toastError(err instanceof Error ? err.message : "Error al cargar catálogo MCC"); })
      .finally(() => setLoading(false));
  }

  useEffect(() => { loadMccs(); }, []);

  function openEdit(m: MCC) {
    setEditForm({ riskLevel: m.riskLevel, networks: [...m.networks], description: m.description });
    setEditingMcc(m);
  }

  async function saveEdit() {
    if (!editingMcc) return;
    try {
      await mccApi.update(editingMcc.id, {
        riskLevel: editForm.riskLevel,
        networks: editForm.networks,
        description: editForm.description,
      });
      toastSuccess(`MCC ${editingMcc.code} actualizado`);
      setEditingMcc(null);
      loadMccs();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Error al actualizar MCC");
    }
  }

  function toggleNetwork(net: MCCNetwork) {
    setEditForm(prev => ({
      ...prev,
      networks: prev.networks.includes(net)
        ? prev.networks.filter(n => n !== net)
        : [...prev.networks, net],
    }));
  }

  const filtered = useMemo(() => {
    return mccs.filter(m => {
      if (selectedCat && m.category !== selectedCat) return false;
      if (selectedRisk && m.riskLevel !== selectedRisk) return false;
      if (search) {
        const q = search.toLowerCase();
        return m.code.includes(q) || m.description.toLowerCase().includes(q) || m.category.toLowerCase().includes(q);
      }
      return true;
    });
  }, [mccs, search, selectedCat, selectedRisk]);

  const stats = useMemo(() => ({
    total: mccs.length,
    high: mccs.filter(m => m.riskLevel === "high").length,
    medium: mccs.filter(m => m.riskLevel === "medium").length,
    low: mccs.filter(m => m.riskLevel === "low").length,
    categories: categories.length,
  }), [mccs, categories]);

  // Group filtered by category
  const grouped = useMemo(() => {
    const map = new Map<string, MCC[]>();
    for (const m of filtered) {
      if (!map.has(m.category)) map.set(m.category, []);
      map.get(m.category)!.push(m);
    }
    return Array.from(map.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [filtered]);

  return (
    <>
      <Header title="Catalogo MCC" />
      <div className="p-6 space-y-6">
        {/* Stats */}
        <div className="grid grid-cols-5 gap-3">
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-primary/10 p-2"><CreditCard className="h-5 w-5 text-primary" /></div>
              <div>
                <p className="text-2xl font-bold">{stats.total}</p>
                <p className="text-xs text-muted-foreground">Total MCCs</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-red-100 dark:bg-red-900/30 p-2"><ShieldAlert className="h-5 w-5 text-red-600 dark:text-red-400" /></div>
              <div>
                <p className="text-2xl font-bold">{stats.high}</p>
                <p className="text-xs text-muted-foreground">Riesgo alto</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-yellow-100 dark:bg-yellow-900/30 p-2"><Shield className="h-5 w-5 text-yellow-600 dark:text-yellow-400" /></div>
              <div>
                <p className="text-2xl font-bold">{stats.medium}</p>
                <p className="text-xs text-muted-foreground">Riesgo medio</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-green-100 dark:bg-green-900/30 p-2"><ShieldCheck className="h-5 w-5 text-green-600 dark:text-green-400" /></div>
              <div>
                <p className="text-2xl font-bold">{stats.low}</p>
                <p className="text-xs text-muted-foreground">Riesgo bajo</p>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="flex items-center gap-3 p-4">
              <div className="rounded-lg bg-purple-100 dark:bg-purple-900/30 p-2"><CreditCard className="h-5 w-5 text-purple-600 dark:text-purple-400" /></div>
              <div>
                <p className="text-2xl font-bold">{stats.categories}</p>
                <p className="text-xs text-muted-foreground">Categorias</p>
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
                placeholder="Buscar por codigo, descripcion o categoria..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <div className="flex gap-1">
              {(["high", "medium", "low"] as const).map(level => {
                const cfg = riskConfig[level];
                return (
                  <button
                    key={level}
                    onClick={() => setSelectedRisk(selectedRisk === level ? "" : level)}
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
              onClick={() => setSelectedCat("")}
              className={cn(
                "rounded-md border px-2.5 py-1 text-xs transition-colors",
                !selectedCat ? "bg-primary text-primary-foreground" : "hover:bg-accent"
              )}
            >Todas ({stats.total})</button>
            {categories.map(c => {
              const count = mccs.filter(m => m.category === c).length;
              return (
                <button
                  key={c}
                  onClick={() => setSelectedCat(selectedCat === c ? "" : c)}
                  className={cn(
                    "rounded-md border px-2.5 py-1 text-xs transition-colors",
                    selectedCat === c ? "bg-primary text-primary-foreground" : "hover:bg-accent"
                  )}
                >{c} ({count})</button>
              );
            })}
          </div>
        </div>

        {/* Results */}
        {loading ? (
          <div className="py-12 text-center text-muted-foreground">Cargando catalogo MCC...</div>
        ) : (
          <div className="space-y-6">
            <p className="text-sm text-muted-foreground">
              Mostrando {filtered.length} de {mccs.length} MCCs
              {selectedCat && <> en <strong>{selectedCat}</strong></>}
              {selectedRisk && <> con riesgo <strong>{riskConfig[selectedRisk]?.label}</strong></>}
            </p>

            {grouped.map(([category, items]) => (
              <div key={category}>
                <div className="mb-2 flex items-center justify-between">
                  <h3 className="text-sm font-semibold">{category}</h3>
                  <Badge variant="secondary" className="text-[10px]">{items.length} MCCs</Badge>
                </div>
                <div className="rounded-lg border overflow-hidden">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b bg-muted/50 text-xs text-muted-foreground">
                        <th className="px-4 py-2 text-left w-20">Codigo</th>
                        <th className="px-4 py-2 text-left">Descripcion</th>
                        <th className="px-4 py-2 text-left w-20">Riesgo</th>
                        <th className="px-4 py-2 text-left w-[180px]">Redes</th>
                        <th className="px-4 py-2 text-left w-10"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {items.map(m => {
                        const risk = riskConfig[m.riskLevel];
                        return (
                          <tr key={m.code} className="border-b last:border-0 hover:bg-muted/30 transition-colors">
                            <td className="px-4 py-2 font-mono text-xs font-medium">{m.code}</td>
                            <td className="px-4 py-2">{m.description}</td>
                            <td className="px-4 py-2">
                              <span className={cn("inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium", risk?.color)}>
                                {risk?.label}
                              </span>
                            </td>
                            <td className="px-4 py-2">
                              {m.networks.length >= 6 ? (
                                <span className="text-[10px] text-muted-foreground font-medium whitespace-nowrap" title={m.networks.map(n => networkLabels[n]?.label).join(", ")}>
                                  Todas
                                </span>
                              ) : (
                                <div className="flex gap-0.5">
                                  {m.networks.map(n => {
                                    const net = networkLabels[n];
                                    return net ? (
                                      <span key={n} title={net.label} className={cn("inline-block rounded px-1 py-0.5 text-[8px] font-bold leading-none whitespace-nowrap", net.color)}>
                                        {net.short}
                                      </span>
                                    ) : null;
                                  })}
                                </div>
                              )}
                            </td>
                            <td className="px-2 py-2">
                              <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEdit(m)} title="Editar">
                                <Pencil className="h-3.5 w-3.5" />
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
                <CreditCard className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
                <p className="font-medium">Sin resultados</p>
                <p className="text-sm text-muted-foreground">Intenta con otra busqueda o filtro</p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Edit Dialog */}
      <Dialog open={!!editingMcc} onOpenChange={(open) => !open && setEditingMcc(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <CreditCard className="h-5 w-5" />
              Editar MCC {editingMcc?.code}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            {/* Description */}
            <div className="space-y-1.5">
              <Label>Descripcion</Label>
              <Input
                value={editForm.description}
                onChange={(e) => setEditForm(prev => ({ ...prev, description: e.target.value }))}
              />
            </div>

            {/* Risk Level */}
            <div className="space-y-1.5">
              <Label>Nivel de riesgo</Label>
              <Select value={editForm.riskLevel} onValueChange={(v) => setEditForm(prev => ({ ...prev, riskLevel: v as MCCRiskLevel }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="high">Alto</SelectItem>
                  <SelectItem value="medium">Medio</SelectItem>
                  <SelectItem value="low">Bajo</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Networks */}
            <div className="space-y-1.5">
              <Label>Redes habilitadas</Label>
              <div className="flex flex-wrap gap-1.5">
                {ALL_NETWORKS.map(net => {
                  const info = networkLabels[net];
                  const active = editForm.networks.includes(net);
                  return (
                    <button
                      key={net}
                      type="button"
                      onClick={() => toggleNetwork(net)}
                      className={cn(
                        "rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors",
                        active ? info.color : "bg-muted/50 text-muted-foreground hover:bg-accent"
                      )}
                    >
                      {info.label}
                    </button>
                  );
                })}
              </div>
              <p className="text-[10px] text-muted-foreground">{editForm.networks.length} de {ALL_NETWORKS.length} redes seleccionadas</p>
            </div>

            {/* Actions */}
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => setEditingMcc(null)}>Cancelar</Button>
              <Button onClick={saveEdit}>Guardar cambios</Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
