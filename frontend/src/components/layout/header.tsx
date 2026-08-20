"use client";

import { Bell, Moon, Sun } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { alertsApi } from "@/lib/api/alerts";
import { useTheme } from "@/components/theme-provider";

export function Header({ title }: { title: string }) {
  const { theme, toggleTheme } = useTheme();
  const [alertCount, setAlertCount] = useState(0);

  useEffect(() => {
    alertsApi.stats().then((s) => setAlertCount(s.new)).catch(() => {});
  }, []);

  return (
    <header className="flex h-14 items-center justify-between border-b px-6">
      <h1 className="text-lg font-semibold">{title}</h1>

      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" className="relative" asChild>
          <a href="/alerts">
            <Bell className="h-4 w-4" />
            {alertCount > 0 && (
              <Badge className="absolute -right-1 -top-1 h-5 w-5 items-center justify-center rounded-full p-0 text-[10px]">
                {alertCount}
              </Badge>
            )}
          </a>
        </Button>

        <Button variant="ghost" size="icon" onClick={toggleTheme}>
          {theme === "light" ? <Moon className="h-4 w-4" /> : <Sun className="h-4 w-4" />}
        </Button>
      </div>
    </header>
  );
}
