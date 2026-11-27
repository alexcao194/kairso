"use client";

import { useFestiveTheme, useReducedMotion } from "@/lib/hooks/festive";
import { useFestiveMusicController } from "@/lib/music/useFestiveMusic";
import { ParticleLayer } from "./ParticleLayer";
import { Garland } from "./Decor";

/** Today's drifting particles over the app; nothing when off, unsupported or reduced motion. */
export function FestiveLayer() {
  const theme = useFestiveTheme();
  const reduced = useReducedMotion();
  useFestiveMusicController(theme);
  if (!theme?.particle || reduced) return null;
  return (
    <div className="print:hidden">
      <ParticleLayer kind={theme.particle} />
    </div>
  );
}

/** Holiday garland along the top of the main area. */
export function FestiveGarland() {
  const theme = useFestiveTheme();
  return theme?.holiday ? <Garland theme={theme.id} /> : null;
}
