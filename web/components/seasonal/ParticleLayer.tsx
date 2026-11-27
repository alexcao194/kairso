"use client";

import { useEffect, useRef } from "react";
import type { Particle } from "@/lib/domain/season";

/**
 * A sparse layer of drifting particles (snow, leaves, petals…) over the whole UI. One canvas,
 * never catches clicks, sits under popovers and dialogs, caps at 30 fps and stops while the tab
 * is hidden. Colours come from the --fx-* tokens, so it follows light/dark.
 */

interface Spec {
  /** Particles per 100k px² of viewport, and an absolute cap. */
  density: number;
  max: number;
  size: [number, number];
  /** Vertical speed in px/s (negative rises), horizontal sway amplitude in px. */
  speed: [number, number];
  sway: number;
  alpha: [number, number];
  colors: string[];
}

const SPECS: Record<Particle, Spec> = {
  snow: { density: 1.6, max: 30, size: [3.5, 7.5], speed: [14, 32], sway: 18, alpha: [0.55, 0.9], colors: ["--fx-snow"] },
  leaf: { density: 0.8, max: 14, size: [6, 10], speed: [18, 34], sway: 34, alpha: [0.55, 0.85], colors: ["--fx-leaf-1", "--fx-leaf-2", "--fx-leaf-3"] },
  petal: { density: 1, max: 18, size: [4, 7], speed: [14, 26], sway: 30, alpha: [0.5, 0.85], colors: ["--fx-petal", "--fx-petal-2"] },
  blossom: { density: 1, max: 18, size: [4.5, 7.5], speed: [14, 26], sway: 30, alpha: [0.6, 0.9], colors: ["--fx-mai", "--fx-dao"] },
  heart: { density: 0.6, max: 12, size: [5, 9], speed: [-22, -12], sway: 16, alpha: [0.35, 0.65], colors: ["--fx-heart", "--fx-petal-2"] },
  confetti: { density: 1.4, max: 26, size: [3, 6], speed: [26, 48], sway: 14, alpha: [0.7, 0.95], colors: ["--rose-vivid", "--amber-vivid", "--blue-vivid", "--green-vivid", "--violet-vivid"] },
  bat: { density: 0.25, max: 5, size: [9, 14], speed: [-14, -6], sway: 40, alpha: [0.45, 0.7], colors: ["--fx-bat"] },
  spark: { density: 0.9, max: 16, size: [1.6, 3], speed: [-12, -5], sway: 22, alpha: [0.35, 0.8], colors: ["--fx-spark"] },
};

interface P {
  x: number; y: number; size: number; vy: number; phase: number; freq: number;
  rot: number; vr: number; alpha: number; color: string;
}

const rand = (a: number, b: number) => a + Math.random() * (b - a);

function readColors(spec: Spec): string[] {
  const cs = getComputedStyle(document.documentElement);
  return spec.colors.map((v) => cs.getPropertyValue(v).trim() || "#999");
}

function spawn(spec: Spec, w: number, h: number, colors: string[], anywhere: boolean): P {
  const vy = rand(spec.speed[0], spec.speed[1]);
  const rising = vy < 0;
  return {
    x: rand(0, w),
    y: anywhere ? rand(0, h) : rising ? h + 20 : -20,
    size: rand(spec.size[0], spec.size[1]),
    vy,
    phase: rand(0, Math.PI * 2),
    freq: rand(0.4, 0.9),
    rot: rand(0, Math.PI * 2),
    vr: rand(-1.2, 1.2),
    alpha: rand(spec.alpha[0], spec.alpha[1]),
    color: colors[Math.floor(Math.random() * colors.length)],
  };
}

function draw(ctx: CanvasRenderingContext2D, kind: Particle, p: P, t: number, gold: string) {
  const s = p.size;
  ctx.save();
  ctx.translate(p.x, p.y);
  ctx.globalAlpha = p.alpha;
  ctx.fillStyle = p.color;
  ctx.strokeStyle = p.color;
  switch (kind) {
    case "snow": {
      // Six-armed snowflake with a pair of side branches on each arm, turning slowly.
      ctx.rotate(p.rot * 0.4);
      ctx.lineWidth = Math.max(0.8, s * 0.16);
      ctx.lineCap = "round";
      ctx.beginPath();
      for (let i = 0; i < 6; i++) {
        const a = (Math.PI / 3) * i;
        const cos = Math.cos(a), sin = Math.sin(a);
        const at = (r: number, side: number) => {
          // A point on the arm at radius r, pushed sideways for the branches.
          const x = cos * r - sin * side, y = sin * r + cos * side;
          return [x, y] as const;
        };
        ctx.moveTo(0, 0);
        ctx.lineTo(cos * s, sin * s);
        const [bx, by] = at(s * 0.55, 0);
        const [lx, ly] = at(s * 0.8, s * 0.26);
        const [rx, ry] = at(s * 0.8, -s * 0.26);
        ctx.moveTo(bx, by);
        ctx.lineTo(lx, ly);
        ctx.moveTo(bx, by);
        ctx.lineTo(rx, ry);
      }
      ctx.stroke();
      break;
    }
    case "spark": {
      ctx.globalAlpha = p.alpha * (0.55 + 0.45 * Math.sin(t * 2.2 + p.phase));
      const g = ctx.createRadialGradient(0, 0, 0, 0, 0, s * 3.2);
      g.addColorStop(0, p.color);
      g.addColorStop(1, "transparent");
      ctx.fillStyle = g;
      ctx.beginPath();
      ctx.arc(0, 0, s * 3.2, 0, Math.PI * 2);
      ctx.fill();
      break;
    }
    case "leaf": {
      ctx.rotate(p.rot);
      ctx.scale(1, 0.55 + 0.45 * Math.abs(Math.sin(t * 1.3 + p.phase))); // tumbling
      ctx.beginPath();
      ctx.moveTo(-s, 0);
      ctx.quadraticCurveTo(0, -s * 0.9, s, 0);
      ctx.quadraticCurveTo(0, s * 0.9, -s, 0);
      ctx.fill();
      break;
    }
    case "petal": {
      ctx.rotate(p.rot);
      ctx.scale(1, 0.6 + 0.4 * Math.abs(Math.sin(t * 1.1 + p.phase)));
      ctx.beginPath();
      ctx.moveTo(0, -s);
      ctx.bezierCurveTo(s * 0.9, -s * 0.6, s * 0.6, s * 0.7, 0, s);
      ctx.bezierCurveTo(-s * 0.6, s * 0.7, -s * 0.9, -s * 0.6, 0, -s);
      ctx.fill();
      break;
    }
    case "blossom": {
      // Five-petal apricot (mai) / peach (đào) flower.
      ctx.rotate(p.rot);
      for (let i = 0; i < 5; i++) {
        ctx.rotate((Math.PI * 2) / 5);
        ctx.beginPath();
        ctx.ellipse(0, -s * 0.55, s * 0.36, s * 0.55, 0, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.fillStyle = gold;
      ctx.beginPath();
      ctx.arc(0, 0, s * 0.18, 0, Math.PI * 2);
      ctx.fill();
      break;
    }
    case "heart": {
      ctx.rotate(Math.sin(t + p.phase) * 0.25);
      ctx.beginPath();
      ctx.moveTo(0, s * 0.35);
      ctx.bezierCurveTo(-s, -s * 0.35, -s * 0.45, -s, 0, -s * 0.45);
      ctx.bezierCurveTo(s * 0.45, -s, s, -s * 0.35, 0, s * 0.35);
      ctx.fill();
      break;
    }
    case "confetti": {
      ctx.rotate(p.rot);
      ctx.scale(Math.cos(t * 3 + p.phase), 1); // flipping
      ctx.fillRect(-s / 2, -s / 4, s, s / 2);
      break;
    }
    case "bat": {
      const flap = 0.35 + 0.65 * Math.abs(Math.sin(t * 7 + p.phase));
      ctx.beginPath();
      ctx.moveTo(0, 0);
      ctx.quadraticCurveTo(-s * 0.5, -s * 0.6 * flap, -s, -s * 0.15 * flap);
      ctx.quadraticCurveTo(-s * 0.7, s * 0.05, -s * 0.55, s * 0.25);
      ctx.quadraticCurveTo(-s * 0.3, s * 0.05, 0, s * 0.3);
      ctx.quadraticCurveTo(s * 0.3, s * 0.05, s * 0.55, s * 0.25);
      ctx.quadraticCurveTo(s * 0.7, s * 0.05, s, -s * 0.15 * flap);
      ctx.quadraticCurveTo(s * 0.5, -s * 0.6 * flap, 0, 0);
      ctx.fill();
      break;
    }
  }
  ctx.restore();
}

/** `factor` scales how many particles spawn (1 = default density). */
export function ParticleLayer({ kind, factor = 1 }: { kind: Particle; factor?: number }) {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext("2d");
    if (!canvas || !ctx) return;
    const spec = SPECS[kind];
    let colors = readColors(spec);
    let gold = getComputedStyle(document.documentElement).getPropertyValue("--fx-gold").trim();
    let w = 0, h = 0;
    let parts: P[] = [];

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      w = window.innerWidth;
      h = window.innerHeight;
      canvas.width = Math.round(w * dpr);
      canvas.height = Math.round(h * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      const n = Math.min(Math.round(spec.max * factor), Math.max(1, Math.round(((w * h) / 100_000) * spec.density * factor)));
      parts = parts.slice(0, n);
      while (parts.length < n) parts.push(spawn(spec, w, h, colors, true));
    };
    resize();

    // Theme switch → new colours.
    const mo = new MutationObserver(() => {
      colors = readColors(spec);
      gold = getComputedStyle(document.documentElement).getPropertyValue("--fx-gold").trim();
      for (const p of parts) p.color = colors[Math.floor(Math.random() * colors.length)];
    });
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme", "style"] });

    let raf = 0;
    let last = performance.now();
    const frame = (now: number) => {
      raf = requestAnimationFrame(frame);
      const dt = (now - last) / 1000;
      if (dt < 1 / 30) return;
      last = now;
      const step = Math.min(dt, 0.1);
      const t = now / 1000;
      ctx.clearRect(0, 0, w, h);
      for (let i = 0; i < parts.length; i++) {
        const p = parts[i];
        p.y += p.vy * step;
        p.x += Math.cos(t * p.freq + p.phase) * spec.sway * step;
        p.rot += p.vr * step;
        if (p.y > h + 30 || p.y < -30 || p.x < -40 || p.x > w + 40) parts[i] = spawn(spec, w, h, colors, false);
        draw(ctx, kind, parts[i], t, gold);
      }
    };
    const start = () => {
      if (!raf && !document.hidden) {
        last = performance.now();
        raf = requestAnimationFrame(frame);
      }
    };
    const stop = () => {
      cancelAnimationFrame(raf);
      raf = 0;
    };
    const onVisibility = () => (document.hidden ? stop() : start());
    start();
    window.addEventListener("resize", resize);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      mo.disconnect();
      window.removeEventListener("resize", resize);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [kind, factor]);

  return <canvas ref={ref} aria-hidden="true" className="pointer-events-none fixed inset-0 z-[25] size-full" />;
}
