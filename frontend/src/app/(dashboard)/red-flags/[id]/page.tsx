"use client";

import { useParams } from "next/navigation";
import { RedFlagTabContent } from "@/components/tabs/content/red-flag-tab-content";

export default function RedFlagCasePage() {
  const { id } = useParams<{ id: string }>();
  return <RedFlagTabContent params={{ id }} tabId={`/red-flags/${id}`} />;
}
