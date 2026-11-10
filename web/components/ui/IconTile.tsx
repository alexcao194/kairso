import type { ReactNode } from "react";
import { hueVars, type Hue } from "@/lib/domain/hue";

/**
 * Ô icon màu. `solid`: gradient đặc + icon trắng (điểm nhấn); mặc định: nền nhạt + icon đậm.
 * Chỉ trang trí — không mang thông tin riêng, nên luôn đi kèm chữ bên cạnh.
 */
export function IconTile({
  hue,
  size = 28,
  solid = false,
  className = "",
  children,
}: {
  hue: Hue;
  size?: number;
  solid?: boolean;
  className?: string;
  children: ReactNode;
}) {
  return (
    <span
      aria-hidden="true"
      style={{
        ...hueVars(hue),
        width: size,
        height: size,
        ...(solid ? { background: "linear-gradient(135deg, var(--hv), color-mix(in srgb, var(--hv) 78%, black))" } : {}),
      }}
      className={`inline-flex shrink-0 items-center justify-center rounded-lg ${
        solid ? "text-white shadow-sm" : "bg-(--hs) text-(--h)"
      } ${className}`}
    >
      {children}
    </span>
  );
}
