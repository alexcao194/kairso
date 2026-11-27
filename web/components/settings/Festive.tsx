"use client";

import { useEffect, useState } from "react";
import { SegmentedControl, Switch } from "@/components/ui/form";
import { themeOn, type ThemeId } from "@/lib/domain/season";
import { DENSITIES, useFestiveDensity, useFestiveEnabled, useFestiveTheme, useFestiveVolume, useReducedMotion } from "@/lib/hooks/festive";
import { PLAYLIST } from "@/lib/music/scores";
import { setFestiveMusic, setFestiveVolume, useFestiveMusicState } from "@/lib/music/useFestiveMusic";
import { useT } from "@/lib/i18n";

const Card = ({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) => (
  <section className="rounded-lg border border-line bg-surface p-4">
    <h2 className="text-[14px] font-semibold">{title}</h2>
    {hint && <p className="mb-3 mt-0.5 text-[12.5px] text-ink-muted">{hint}</p>}
    <div className={hint ? "" : "mt-3"}>{children}</div>
  </section>
);

/** Settings → Effects & music: seasonal decorations, particle density and background music. */
export function FestiveSection() {
  const t = useT().festive;
  const [enabled, setEnabled] = useFestiveEnabled();
  const [density, setDensity] = useFestiveDensity();
  const [volume, setVolume] = useFestiveVolume();
  const reduced = useReducedMotion();
  const music = useFestiveMusicState();
  const active = useFestiveTheme();
  // Today's theme even while effects are off, for the label and playlist.
  const [today, setToday] = useState<ThemeId | null>(null);
  useEffect(() => setToday(themeOn(new Date()).id), []);
  const themeId = active?.id ?? today;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[12.5px] text-ink-muted">{t.localNote}</p>

      <Card title={t.effects} hint={t.effectsHint}>
        <div className="flex items-center justify-between gap-3">
          <span className="text-[13px] text-ink">{t.effectsLabel}</span>
          <Switch checked={enabled} onChange={setEnabled} label={t.effectsLabel} />
        </div>
        {themeId && <p className="mt-2 text-[12.5px] text-ink-muted">{t.today(t.themeName[themeId])}</p>}
      </Card>

      <Card title={t.density} hint={t.densityHint}>
        <SegmentedControl
          label={t.density}
          value={density}
          onChange={setDensity}
          options={DENSITIES.map((d) => ({ value: d, label: t.densities[d] }))}
        />
        {reduced && <p className="mt-2 text-[12.5px] text-warning">{t.reducedMotion}</p>}
        {!enabled && <p className="mt-2 text-[12.5px] text-ink-faint">{t.offHint}</p>}
      </Card>

      <Card title={t.music} hint={t.musicHint}>
        <div className="flex items-center justify-between gap-3">
          <span className="text-[13px] text-ink">{t.musicLabel}</span>
          <span data-festive-music>
            <Switch checked={music.on} disabled={!enabled} onChange={setFestiveMusic} label={t.musicLabel} />
          </span>
        </div>
        <label className="mt-3 flex items-center gap-3 text-[13px] text-ink-2">
          <span className="w-20 shrink-0">{t.volume}</span>
          <input
            type="range"
            min={0}
            max={100}
            step={5}
            value={volume}
            onChange={(e) => {
              const v = Number(e.target.value);
              setVolume(v);
              setFestiveVolume(v);
            }}
            className="h-1 flex-1 cursor-pointer accent-[var(--accent)]"
          />
          <span className="w-10 text-right font-mono text-[12px] text-ink-muted">{volume}%</span>
        </label>
        {music.on && music.track && <p className="mt-2 text-[12.5px] text-accent-ink">{t.nowPlaying(music.track.title, music.track.composer)}</p>}
        {themeId && (
          <div className="mt-3">
            <p className="text-[12px] font-medium text-ink-muted">{t.playlist}</p>
            <ul className="mt-1 flex flex-col gap-0.5 text-[12.5px] text-ink-2">
              {PLAYLIST[themeId].map((s) => (
                <li key={s.title}>
                  {s.title} <span className="text-ink-faint">— {s.composer}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </Card>

    </div>
  );
}
