"use client";

import { useEffect, useState, useMemo } from "react";
import { Header } from "@/components/layout/header";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ScrollText, LogIn, Bell, FileText, Upload, Users, Loader2, CreditCard } from "lucide-react";
import { activityLogsApi } from "@/lib/api/activity-logs";
import { useToast } from "@/lib/use-toast";
import { formatDate } from "@/lib/utils";
import { CATEGORY_CLASSES } from "@/lib/semantic-colors";
import type { ActivityLogEntry, ActivityType } from "@/lib/types";

const ACTION_LABELS: Record<ActivityType, string> = {
  login: "Inicio de sesión",
  red_flag_action: "Acción en bandera roja",
  rule_create: "Regla creada",
  rule_update: "Regla actualizada",
  rule_delete: "Regla eliminada",
  rule_execute: "Regla ejecutada",
  upload: "Carga de datos",
  user_manage: "Gestión de usuario",
  mcc_update: "Actualización de MCC",
};

const ACTION_ICONS: Record<ActivityType, typeof LogIn> = {
  login: LogIn,
  red_flag_action: Bell,
  rule_create: FileText,
  rule_update: FileText,
  rule_delete: FileText,
  rule_execute: FileText,
  upload: Upload,
  user_manage: Users,
  mcc_update: CreditCard,
};

// Tipo de acción: eje categórico. Distingue sin ordenar ni medir severidad.
// Hay 8 series y 9 acciones, así que dos comparten color: se eligen dos que
// rara vez aparecen juntas en la misma pantalla.
const ACTION_COLORS: Record<ActivityType, string> = {
  login: CATEGORY_CLASSES[0],
  red_flag_action: CATEGORY_CLASSES[2],
  rule_create: CATEGORY_CLASSES[1],
  rule_update: CATEGORY_CLASSES[3],
  rule_delete: CATEGORY_CLASSES[5],
  rule_execute: CATEGORY_CLASSES[4],
  upload: CATEGORY_CLASSES[6],
  user_manage: CATEGORY_CLASSES[7],
  mcc_update: CATEGORY_CLASSES[2],
};

export default function ActivityLogsPage() {
  const [logs, setLogs] = useState<ActivityLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterAction, setFilterAction] = useState<string>("all");
  const [filterUser, setFilterUser] = useState<string>("all");
  const { toastError } = useToast();

  useEffect(() => { loadLogs(); }, []);

  async function loadLogs() {
    setLoading(true);
    try {
      setLogs(await activityLogsApi.list({ limit: 200 }));
    } catch {
      toastError("Error al cargar bitácora de acceso");
    } finally {
      setLoading(false);
    }
  }

  const uniqueUsers = useMemo(() => {
    const map = new Map<string, string>();
    for (const log of logs) {
      map.set(log.userId, log.userName);
    }
    return Array.from(map.entries());
  }, [logs]);

  const filtered = useMemo(() => {
    let result = logs;
    if (filterAction !== "all") result = result.filter((l) => l.action === filterAction);
    if (filterUser !== "all") result = result.filter((l) => l.userId === filterUser);
    return result;
  }, [logs, filterAction, filterUser]);

  return (
    <>
      <Header title="Bitácora de acceso" />
      <div className="p-6 space-y-4">
        <div className="flex gap-3 flex-wrap">
          <Select value={filterAction} onValueChange={setFilterAction}>
            <SelectTrigger className="w-48"><SelectValue placeholder="Tipo de acción" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todas las acciones</SelectItem>
              <SelectItem value="login">Inicio de sesión</SelectItem>
              <SelectItem value="red_flag_action">Acción en bandera roja</SelectItem>
              <SelectItem value="rule_create">Regla creada</SelectItem>
              <SelectItem value="rule_update">Regla actualizada</SelectItem>
              <SelectItem value="rule_delete">Regla eliminada</SelectItem>
              <SelectItem value="rule_execute">Regla ejecutada</SelectItem>
              <SelectItem value="upload">Carga de datos</SelectItem>
              <SelectItem value="user_manage">Gestión de usuario</SelectItem>
              <SelectItem value="mcc_update">Actualización de MCC</SelectItem>
            </SelectContent>
          </Select>
          <Select value={filterUser} onValueChange={setFilterUser}>
            <SelectTrigger className="w-48"><SelectValue placeholder="Usuario" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los usuarios</SelectItem>
              {uniqueUsers.map(([id, name]) => (
                <SelectItem key={id} value={id}>{name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Badge variant="outline" className="h-9 px-3 flex items-center">
            {filtered.length} registros
          </Badge>
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        ) : filtered.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-12">
              <ScrollText className="mb-4 h-12 w-12 text-muted-foreground" />
              <p className="font-medium">No hay actividad registrada</p>
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {filtered.map((log) => {
              const Icon = ACTION_ICONS[log.action] || ScrollText;
              const colorClass = ACTION_COLORS[log.action] || "bg-surface-2 text-ink-muted";
              return (
                <Card key={log.id}>
                  <CardContent className="flex items-center gap-4 p-4">
                    <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${colorClass}`}>
                      <Icon className="h-4 w-4" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="font-medium text-sm">{log.userName}</span>
                        <Badge variant="outline" className="text-xs">
                          {ACTION_LABELS[log.action] || log.action}
                        </Badge>
                      </div>
                      <p className="text-sm text-muted-foreground truncate">{log.detail}</p>
                    </div>
                    <div className="text-right shrink-0">
                      <p className="text-xs text-muted-foreground">{formatDate(log.createdAt)}</p>
                      <p className="text-xs text-muted-foreground">{log.ip}</p>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </div>
    </>
  );
}
