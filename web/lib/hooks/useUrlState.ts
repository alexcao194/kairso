"use client";

import { useCallback } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

/**
 * Trạng thái view (tab, bộ lọc, nhóm, sắp xếp…) sống trong query string để chia sẻ được link.
 * Giá trị bằng mặc định thì bỏ khỏi URL cho gọn. Dùng trong component nằm dưới <Suspense>.
 */
export function useUrlState(key: string, def = ""): [string, (v: string | null) => void] {
  const sp = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const value = sp.get(key) ?? def;

  const set = useCallback(
    (v: string | null) => {
      const next = new URLSearchParams(window.location.search);
      if (v === null || v === "" || v === def) next.delete(key);
      else next.set(key, v);
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [key, def, router, pathname],
  );
  return [value, set];
}
