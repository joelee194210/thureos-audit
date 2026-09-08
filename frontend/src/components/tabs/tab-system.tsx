"use client";

import { usePathname } from "next/navigation";
import { TabBar } from "./tab-bar";
import { TabContentHost } from "./tab-content-host";
import { TAB_CONTENT_REGISTRY } from "@/lib/tab-registry";
import { labelForPath } from "@/lib/tab-labels";

export function TabSystem({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const label = labelForPath(pathname);

  return (
    <>
      <TabBar />
      <main className="flex-1 overflow-y-auto">
        <TabContentHost
          currentPath={pathname}
          currentLabel={label}
          registry={TAB_CONTENT_REGISTRY}
        >
          {children}
        </TabContentHost>
      </main>
    </>
  );
}
