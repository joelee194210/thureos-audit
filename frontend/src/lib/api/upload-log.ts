import { api } from "./client";
import type { UploadLogEntry } from "@/lib/types";

export const uploadLogApi = {
  list: () => api.get<UploadLogEntry[]>("/upload-log"),

  approve: (monitorId: string, logId: string) =>
    api.post<{ recordsIngested: number; rowsRejected: number }>(
      `/monitors/${monitorId}/upload-log/${logId}/approve`,
      {},
    ),
};
