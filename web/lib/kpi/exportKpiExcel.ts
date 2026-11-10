import { requestBlob } from "@/lib/api/http";

/**
 * Xuất phiếu KPI dạng Excel. File do **backend** dựng (gokairso performance,
 * endpoint GET /kpi/export) — trình duyệt chỉ tải blob về và lưu, không còn dựng
 * .xlsx tại chỗ. Proxy không chuyển tiếp Content-Disposition nên tên file đặt ở đây.
 */
function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

const fileLabel = (month: string) => month.replace("/", "-"); // "2026-10" hoặc "10/2026" → an toàn cho tên file

/** Xuất phiếu KPI của 1 thành viên (1 sheet). `month` dạng YYYY-MM. */
export async function exportOneKpiExcel(workspaceId: string, month: string, memberId: string, alias?: string) {
  const blob = await requestBlob("GET", `/v1/workspaces/${workspaceId}/kpi/export`, { query: { month, memberId } });
  triggerDownload(blob, `KPI_${alias || memberId}_${fileLabel(month)}.xlsx`);
}

/** Xuất phiếu KPI cả nhóm — mỗi thành viên một sheet. `month` dạng YYYY-MM. */
export async function exportGroupKpiExcel(workspaceId: string, month: string) {
  const blob = await requestBlob("GET", `/v1/workspaces/${workspaceId}/kpi/export`, { query: { month } });
  triggerDownload(blob, `KPI_${fileLabel(month)}.xlsx`);
}
