"use client";

import { useParams } from "next/navigation";
import { MonitorTabContent } from "@/components/tabs/content/monitor-tab-content";

export default function MonitorDetailPage() {
  const { id } = useParams<{ id: string }>();
  return <MonitorTabContent params={{ id }} tabId={`/monitors/${id}`} />;
}
