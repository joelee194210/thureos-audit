import type { LucideIcon } from "lucide-react";
import {
  LayoutDashboard, Bell, Upload, Monitor, ShieldCheck, ScanSearch,
  BarChart3, Bot, CreditCard, Globe, Users, ScrollText, Settings,
  CircleHelp,
} from "lucide-react";
import type { Role } from "@/lib/types";

export interface NavItem {
  name: string;
  href: string;
  icon: LucideIcon;
  roles?: Role[];
}

export interface NavArea {
  label: string;
  items: NavItem[];
  roles?: Role[];
}

export const NAV_AREAS: NavArea[] = [
  {
    label: "Operación",
    items: [
      { name: "Dashboard", href: "/", icon: LayoutDashboard },
      { name: "Banderas rojas", href: "/red-flags", icon: Bell },
      { name: "Cargas", href: "/uploads", icon: Upload },
      { name: "Ayuda", href: "/ayuda", icon: CircleHelp },
    ],
  },
  {
    label: "Monitoreo",
    items: [
      { name: "Monitores", href: "/monitors", icon: Monitor },
      { name: "Reglas", href: "/rules", icon: ShieldCheck },
      { name: "Screening", href: "/screening", icon: ScanSearch },
      { name: "Dashboards", href: "/dashboards", icon: BarChart3 },
      { name: "Analista IA", href: "/chatbot", icon: Bot },
    ],
  },
  {
    label: "Catálogos",
    items: [
      { name: "MCC", href: "/mcc", icon: CreditCard },
      { name: "Países", href: "/countries", icon: Globe },
    ],
  },
  {
    label: "Administración",
    roles: ["admin"],
    items: [
      { name: "Usuarios", href: "/users", icon: Users },
      { name: "Bitácora de acceso", href: "/activity-logs", icon: ScrollText },
      { name: "Configuración", href: "/settings", icon: Settings },
    ],
  },
];
