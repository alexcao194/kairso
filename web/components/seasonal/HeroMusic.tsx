"use client";

import { IconMusicOff } from "@/components/icons";
import { useFestiveVolume } from "@/lib/hooks/festive";
import { setFestiveVolume, toggleFestiveMusic, useFestiveMusicState } from "@/lib/music/useFestiveMusic";
import { useT } from "@/lib/i18n";
import { credit } from "@/lib/music/scores";

// Static class names so Tailwind generates them: three bars bouncing out of step.
const BARS = [
  "motion-safe:animate-[fx-eq_0.9s_ease-in-out_infinite]",
  "motion-safe:animate-[fx-eq_0.7s_ease-in-out_-0.3s_infinite]",
  "motion-safe:animate-[fx-eq_1.1s_ease-in-out_-0.6s_infinite]",
];

/** Play/stop button for the festive music, styled for the brand-coloured hero banner. */
export function HeroMusicToggle({ withLabel = false }: { withLabel?: boolean }) {
  const music = useFestiveMusicState();
  const t = useT().festive;
  const label = music.on ? t.musicOff : t.musicOn;
  return (
    <button
      type="button"
      data-festive-music
      onClick={toggleFestiveMusic}
      aria-pressed={music.on}
      aria-label={label}
      title={music.on && music.track ? t.nowPlaying(music.track.title, credit(music.track)) : label}
      className={`inline-flex h-ctl shrink-0 items-center justify-center gap-1.5 rounded-lg border border-white/40 bg-white/15 text-[12.5px] font-medium text-white backdrop-blur transition hover:bg-white/25 ${
        withLabel ? "px-3" : "w-ctl"
      }`}
    >
      {music.on ? (
        <span className="flex h-3.5 items-end gap-[2.5px]" aria-hidden="true">
          {BARS.map((c) => (
            <span key={c} className={`h-full w-[2.5px] origin-bottom rounded-full bg-white ${c}`} />
          ))}
        </span>
      ) : (
        <IconMusicOff size={15} />
      )}
      {withLabel && <span>{music.on ? t.musicPlaying : t.musicPlay}</span>}
    </button>
  );
}

/** Music controls under the hero illustration: play/stop and a volume slider. */
export function HeroMusic() {
  const music = useFestiveMusicState();
  const [volume, setVolume] = useFestiveVolume();
  const t = useT().festive;
  return (
    <div data-festive-music className="flex items-center gap-2">
      <HeroMusicToggle withLabel />
      <input
        type="range"
        min={0}
        max={100}
        step={5}
        value={volume}
        aria-label={t.volume}
        title={`${t.volume} ${volume}%`}
        disabled={!music.on}
        onChange={(e) => {
          const v = Number(e.target.value);
          setVolume(v);
          setFestiveVolume(v);
        }}
        className="h-1 w-20 cursor-pointer accent-white disabled:cursor-not-allowed disabled:opacity-40"
      />
    </div>
  );
}
