/** Vài phép thống kê mô tả dùng chung cho các trang Thống kê/Phân tích — tính thẳng từ
 * dữ liệu đã có ở client, không cần thêm API. */

interface WorkloadBalance {
  mean: number;
  stdDev: number;
  /** Hệ số biến thiên (stdDev/mean) — 0 nghĩa là chia đều tuyệt đối, càng lớn càng lệch. */
  cv: number;
  min: { key: string; label: string; value: number } | null;
  max: { key: string; label: string; value: number } | null;
}

/** Độ lệch trong cách chia việc giữa các thành viên — dùng độ lệch chuẩn mẫu (population)
 * và hệ số biến thiên (CV) để so sánh được giữa các kỳ có tổng khối lượng khác nhau. */
export function workloadBalance(rows: Array<{ key: string; label: string; value: number }>): WorkloadBalance {
  if (rows.length === 0) return { mean: 0, stdDev: 0, cv: 0, min: null, max: null };
  const mean = rows.reduce((a, r) => a + r.value, 0) / rows.length;
  const variance = rows.reduce((a, r) => a + (r.value - mean) ** 2, 0) / rows.length;
  const stdDev = Math.sqrt(variance);
  let min = rows[0];
  let max = rows[0];
  for (const r of rows) {
    if (r.value < min.value) min = r;
    if (r.value > max.value) max = r;
  }
  return { mean, stdDev, cv: mean > 0 ? stdDev / mean : 0, min, max };
}

/** Gom giá trị vào các khoảng [lo, hi) liên tiếp — dùng vẽ histogram phân bố KPI. */
export function histogram(values: number[], edges: number[]): number[] {
  const counts = new Array(edges.length - 1).fill(0) as number[];
  for (const v of values) {
    for (let i = 0; i < edges.length - 1; i++) {
      const isLast = i === edges.length - 2;
      if (v >= edges[i] && (isLast ? v <= edges[i + 1] : v < edges[i + 1])) {
        counts[i]++;
        break;
      }
    }
  }
  return counts;
}
