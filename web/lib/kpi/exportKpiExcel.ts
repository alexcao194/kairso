import { requestBlob } from "@/lib/api/http";
import { triggerDownload } from "@/lib/download";

const fileLabel = (month: string) => month.replace("/", "-");

export async function exportOneKpiExcel(workspaceId: string, month: string, memberId: string, alias?: string) {
  const blob = await requestBlob("GET", `/v1/workspaces/${workspaceId}/kpi/export`, { query: { month, memberId } });
  triggerDownload(blob, `KPI_${alias || memberId}_${fileLabel(month)}.xlsx`);
}

export async function exportGroupKpiExcel(workspaceId: string, month: string) {
  const blob = await requestBlob("GET", `/v1/workspaces/${workspaceId}/kpi/export`, { query: { month } });
  triggerDownload(blob, `KPI_${fileLabel(month)}.xlsx`);
}
