"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { TopNav } from "@/components/layout/top-nav";
import { TabSystem } from "@/components/tabs/tab-system";
import { useAuthStore } from "@/stores/auth-store";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const { user, loadUser, token } = useAuthStore();

  useEffect(() => {
    if (!token) {
      router.push("/login");
      return;
    }
    if (!user) {
      loadUser();
    }
  }, [token, user, loadUser, router]);

  if (!token || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="flex h-screen flex-col overflow-hidden">
      <TopNav />
      <TabSystem>{children}</TabSystem>
    </div>
  );
}
