"use client";

import { useEffect, useState } from "react";
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
  ChevronDown,
  Moon,
  Sun,
  ScanSearch,
  type LucideIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Logo } from "@/components/brand/logo";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAuthStore } from "@/stores/auth-store";
import { useTheme } from "@/components/theme-provider";
import { redFlagsApi } from "@/lib/api/red-flags";
import type { Role } from "@/lib/types";

interface NavItem {
  name: string;
  href: string;
  icon: LucideIcon;
}

interface NavArea {
  label: string;
  items: NavItem[];
  roles?: Role[];
}

const areas: NavArea[] = [
  {
    label: "Operación",
    items: [
      { name: "Dashboard", href: "/", icon: LayoutDashboard },
      { name: "Banderas rojas", href: "/red-flags", icon: Bell },
      { name: "Cargas", href: "/uploads", icon: Upload },
    ],
  },
  {
    label: "Monitoreo",
    items: [
      { name: "Monitores", href: "/monitors", icon: Monitor },
      { name: "Reglas", href: "/rules", icon: ShieldCheck },
      { name: "Screening", href: "/screening", icon: ScanSearch },
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

function isItemActive(href: string, pathname: string) {
  return pathname === href || (href !== "/" && pathname.startsWith(href));
}

export function TopNav() {
  const pathname = usePathname();
  const { user, logout } = useAuthStore();
  const { theme, toggleTheme } = useTheme();
  const [redFlagCount, setRedFlagCount] = useState(0);

  useEffect(() => {
    redFlagsApi.stats().then((s) => setRedFlagCount(s.new)).catch(() => {});
  }, []);

  const visibleAreas = areas.filter(
    (area) => !area.roles || (user && area.roles.includes(user.role))
  );

  return (
    <header className="shrink-0 bg-card">
      <div className="h-[3px] bg-[linear-gradient(to_right,var(--thu-navy-700),var(--color-primary),var(--thu-blue-500))]" />
      <div className="flex h-14 items-center gap-1 border-b px-4 md:px-6">
        <Link href="/" className="mr-4 flex items-center gap-2.5 shrink-0">
          <Logo variant="isotipo" size={26} />
          <span className="hidden text-sm font-semibold uppercase tracking-[0.14em] text-ink sm:inline">
            Thureos
          </span>
        </Link>

        <nav className="flex h-full items-stretch overflow-x-auto" aria-label="Áreas">
          {visibleAreas.map((area) => {
            const activeItem = area.items.find((item) => isItemActive(item.href, pathname));
            const isActive = !!activeItem;

            if (area.items.length === 1) {
              const item = area.items[0];
              return (
                <Link
                  key={area.label}
                  href={item.href}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "relative flex items-center gap-2 px-3 text-[13.5px] font-medium transition-colors whitespace-nowrap",
                    isActive ? "text-accent-fg" : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  <item.icon className="h-4 w-4 shrink-0" />
                  {item.name}
                  {isActive && (
                    <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full bg-primary" />
                  )}
                </Link>
              );
            }

            return (
              <DropdownMenu key={area.label}>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    aria-current={isActive ? "page" : undefined}
                    className={cn(
                      "group/area relative flex items-center gap-2 px-3 text-[13.5px] font-medium outline-none transition-colors whitespace-nowrap",
                      isActive ? "text-accent-fg" : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    {activeItem ? <activeItem.icon className="h-4 w-4 shrink-0" /> : null}
                    {area.label}
                    <ChevronDown className="h-3.5 w-3.5 shrink-0 opacity-60 transition-transform duration-200 group-data-[state=open]/area:rotate-180" />
                    {isActive && (
                      <span className="absolute inset-x-3 -bottom-px h-0.5 rounded-full bg-primary" />
                    )}
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="min-w-56">
                  <DropdownMenuLabel>{area.label}</DropdownMenuLabel>
                  {area.items.map((item) => {
                    const itemActive = isItemActive(item.href, pathname);
                    return (
                      <DropdownMenuItem key={item.href} asChild className={cn(itemActive && "bg-accent-soft text-accent-fg")}>
                        <Link href={item.href}>
                          <item.icon className="h-4 w-4 shrink-0" />
                          {item.name}
                        </Link>
                      </DropdownMenuItem>
                    );
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
            );
          })}
        </nav>

        <div className="flex-1" />

        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon" className="relative" asChild>
            <Link href="/red-flags">
              <Bell className="h-4 w-4" />
              {redFlagCount > 0 && (
                <Badge className="absolute -right-1 -top-1 h-5 w-5 items-center justify-center rounded-full p-0 text-[10px]">
                  {redFlagCount}
                </Badge>
              )}
            </Link>
          </Button>

          <Button variant="ghost" size="icon" onClick={toggleTheme}>
            {theme === "light" ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="ml-1 h-9 gap-2 px-2">
                <div className="flex h-6 w-6 items-center justify-center rounded-full bg-accent-soft text-xs font-semibold text-accent-fg">
                  {user?.name?.charAt(0).toUpperCase()}
                </div>
                <span className="hidden max-w-32 truncate text-sm font-medium md:inline">
                  {user?.name}
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-48">
              <DropdownMenuLabel>
                <p className="truncate font-medium">{user?.name}</p>
                <p className="truncate text-xs font-normal text-muted-foreground">{user?.role}</p>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={logout} className="text-danger-fg focus:text-danger-fg">
                <LogOut className="h-4 w-4" />
                Cerrar sesión
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>
    </header>
  );
}
