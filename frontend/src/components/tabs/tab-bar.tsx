"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { X } from "lucide-react";
import { useTabStore } from "@/stores/tab-store";
import { resolveTabLabel } from "@/lib/tab-labels";

export function TabBar() {
  const pathname = usePathname();
  const router = useRouter();
  const tabs = useTabStore((s) => s.tabs);
  const activeTabId = useTabStore((s) => s.activeTabId);
  const entityLabels = useTabStore((s) => s.entityLabels);
  const activateTab = useTabStore((s) => s.activateTab);
  const closeTab = useTabStore((s) => s.closeTab);

  if (tabs.length === 0) return null;

  return (
    <div className="flex shrink-0 items-stretch gap-0.5 overflow-x-auto border-b bg-muted/40 px-2 pt-2">
      {tabs.map((tab) => {
        const isActive = tab.id === activeTabId;
        const label = resolveTabLabel(tab, entityLabels);
        return (
          <div
            key={tab.id}
            className={`group flex shrink-0 items-center gap-2 rounded-t-md border border-b-0 max-w-[180px] ${
              isActive
                ? "border-border bg-accent text-accent-foreground"
                : "border-transparent bg-muted/60 text-muted-foreground shadow-sm hover:bg-accent/50 hover:shadow"
            }`}
          >
            <Link
              href={tab.path}
              onClick={() => activateTab(tab.id)}
              aria-current={isActive ? "page" : undefined}
              className="min-w-0 flex-1 truncate px-3 py-1.5 text-xs font-medium"
            >
              {label}
            </Link>
            <button
              onClick={() => {
                closeTab(tab.id);
                const nextActiveId = useTabStore.getState().activeTabId;
                if (nextActiveId && nextActiveId !== pathname) {
                  router.push(nextActiveId);
                }
              }}
              aria-label={`Cerrar pestaña ${label}`}
              className={`mr-1.5 shrink-0 rounded-sm p-0.5 opacity-0 group-hover:opacity-100 ${
                isActive ? "hover:bg-accent-foreground/10" : "hover:bg-muted"
              }`}
            >
              <X className="h-3 w-3" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
