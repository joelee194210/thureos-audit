"use client";

import { useEffect, useRef, useState } from "react";
import { useTabStore } from "@/stores/tab-store";
import { labelForPath } from "@/lib/tab-labels";
import { matchRegistryEntry, type RegistryEntry } from "@/lib/tab-registry";

function isModifiedClick(e: MouseEvent): boolean {
  return e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey;
}

function findAnchor(target: EventTarget | null): HTMLAnchorElement | null {
  if (!(target instanceof Element)) return null;
  return target.closest("a");
}

/** Registra cualquier navegación real de Next como tab, e intercepta
 * clicks en links a rutas registradas para abrirlas "cero recarga" (sin
 * ir al servidor, vía history.pushState). */
export function useTabNavigation(
  currentPath: string,
  currentLabel: string,
  registry: RegistryEntry[],
) {
  const openTab = useTabStore((s) => s.openTab);
  const lastOpenedPath = useRef<string | null>(null);
  const [visiblePath, setVisiblePath] = useState<string | null>(null);

  useEffect(() => {
    if (lastOpenedPath.current === currentPath) return;
    lastOpenedPath.current = currentPath;
    openTab({ path: currentPath, label: currentLabel });
    setVisiblePath(null);
  }, [currentPath, currentLabel, openTab]);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (isModifiedClick(e)) return;
      const anchor = findAnchor(e.target);
      if (!anchor) return;

      const url = new URL(anchor.href, window.location.origin);
      if (url.origin !== window.location.origin) return;

      const match = matchRegistryEntry(url.pathname, registry);
      if (!match) return;

      // NO stopPropagation: no debe romper onClick propios de otros <a>
      // (ej. un botón que cierra un popover en su propio onClick).
      e.preventDefault();
      openTab({ path: url.pathname, label: labelForPath(url.pathname) });
      window.history.pushState(null, "", url.pathname + url.search + url.hash);
      lastOpenedPath.current = url.pathname;
      setVisiblePath(url.pathname);
    }

    document.addEventListener("click", handleClick, { capture: true });
    return () => document.removeEventListener("click", handleClick, { capture: true });
  }, [openTab, registry]);

  return { visiblePath };
}
