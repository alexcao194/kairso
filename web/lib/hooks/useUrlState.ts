"use client";

import { useCallback } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

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

/** Update several query params in one navigation (null/"" removes a key). */
export function useSetUrlParams() {
  const router = useRouter();
  const pathname = usePathname();
  return useCallback(
    (patch: Record<string, string | null>) => {
      const next = new URLSearchParams(window.location.search);
      for (const [k, v] of Object.entries(patch)) {
        if (v === null || v === "") next.delete(k);
        else next.set(k, v);
      }
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [router, pathname],
  );
}
