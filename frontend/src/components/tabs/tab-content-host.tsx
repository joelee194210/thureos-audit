"use client";

import { useTabStore } from "@/stores/tab-store";
import { matchRegistryEntry, type RegistryEntry } from "@/lib/tab-registry";
import { useTabNavigation } from "./use-tab-navigation";

export function TabContentHost({
  currentPath,
  currentLabel,
  registry,
  children,
}: {
  currentPath: string;
  currentLabel: string;
  registry: RegistryEntry[];
  children: React.ReactNode;
}) {
  const { visiblePath } = useTabNavigation(currentPath, currentLabel, registry);
  const tabs = useTabStore((s) => s.tabs);

  const registeredOpenTabs = tabs
    .map((tab) => {
      const match = matchRegistryEntry(tab.path, registry);
      return match ? { tab, ...match } : null;
    })
    .filter((x): x is NonNullable<typeof x> => x !== null);

  return (
    <>
      <div hidden={visiblePath !== null}>{children}</div>
      {registeredOpenTabs.map(({ tab, entry, params }) => (
        <div key={tab.path} hidden={visiblePath !== tab.path}>
          <entry.Component params={params} tabId={tab.id} />
        </div>
      ))}
    </>
  );
}
