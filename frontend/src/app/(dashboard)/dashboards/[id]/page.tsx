"use client";

import { useParams } from "next/navigation";
import { DashboardTabContent } from "@/components/tabs/content/dashboard-tab-content";

export default function DashboardDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <DashboardTabContent params={{ id }} tabId={`/dashboards/${id}`} />;
}
