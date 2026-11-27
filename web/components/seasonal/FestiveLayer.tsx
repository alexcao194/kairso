"use client";

import { useFestiveTheme, useReducedMotion } from "@/lib/hooks/festive";
import { ParticleLayer } from "./ParticleLayer";

/** Today's drifting particles over the app; nothing when off, unsupported or reduced motion. */
export function FestiveLayer() {
  const theme = useFestiveTheme();
  const reduced = useReducedMotion();
  if (!theme?.particle || reduced) return null;
  return (
    <div className="print:hidden">
      <ParticleLayer kind={theme.particle} />
    </div>
  );
}
