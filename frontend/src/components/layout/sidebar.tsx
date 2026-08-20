"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Monitor,
  ShieldCheck,
  Bell,
  BarChart3,
  Users,
  Settings,
  LogOut,
  CreditCard,
  Globe,
  Upload,
  ScrollText,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Logo } from "@/components/brand/logo";
import { useAuthStore } from "@/stores/auth-store";
import type { Role } from "@/lib/types";

interface NavItem {
  name: string;
  href: string;
  icon: LucideIcon;
}

interface NavSection {
  label: string;
  items: NavItem[];
  /** If set, only these roles see this section */
  roles?: Role[];
}

const sections: NavSection[] = [
  {
    label: "Operación",
    items: [
      { name: "Dashboard", href: "/", icon: LayoutDashboard },
      { name: "Alertas", href: "/alerts", icon: Bell },
      { name: "Cargas", href: "/uploads", icon: Upload },
    ],
  },
  {
    label: "Monitoreo",
    items: [
      { name: "Monitores", href: "/monitors", icon: Monitor },
      { name: "Reglas", href: "/rules", icon: ShieldCheck },
      { name: "Dashboards", href: "/dashboards", icon: BarChart3 },
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

export function Sidebar() {
  const pathname = usePathname();
  const { user, logout } = useAuthStore();

  return (
    <aside className="flex h-screen w-64 flex-col border-r border-line-subtle bg-surface text-ink">
      {/* Logo */}
      <div className="flex h-14 items-center border-b border-line-subtle px-6">
        <Link href="/" className="flex items-center gap-2.5">
          <Logo variant="isotipo" size={28} />
          <span className="text-sm font-semibold uppercase tracking-[0.14em] text-ink">
            Thureos
          </span>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
        {sections.map((section) => {
          // Skip section if role-restricted and user doesn't match
          if (section.roles && (!user || !section.roles.includes(user.role))) {
            return null;
          }

          return (
            <div key={section.label} className="mb-1">
              <p className="mb-2 mt-4 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground first:mt-0">
                {section.label}
              </p>
              {section.items.map((item) => {
                const isActive =
                  pathname === item.href || (item.href !== "/" && pathname.startsWith(item.href));
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                      // El acento sólido se reserva para la acción primaria de cada
                      // vista. La navegación activa usa la variante suave.
                      isActive
                        ? "bg-accent-soft text-accent-fg"
                        : "text-ink-muted hover:bg-surface-2 hover:text-ink"
                    )}
                  >
                    <item.icon className="h-4 w-4" />
                    {item.name}
                  </Link>
                );
              })}
            </div>
          );
        })}
      </nav>

      {/* User */}
      <div className="border-t p-4">
        <div className="flex items-center justify-between">
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">{user?.name}</p>
            <p className="truncate text-xs text-muted-foreground">{user?.role}</p>
          </div>
          <button
            onClick={logout}
            className="rounded-md p-2 text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  );
}
