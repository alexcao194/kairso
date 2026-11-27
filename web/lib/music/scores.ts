import type { ThemeId } from "@/lib/domain/season";

/**
 * Background music for the festive themes: classic pieces in freely licensed recordings from
 * Wikimedia Commons (web/public/music, credits in CREDITS.md there), each optionally backed by a
 * note-list arrangement synthesised in the browser (lib/music/player.ts) if the file can't play.
 *
 * Notation of the arrangements: space-separated `NOTE:beats` tokens. NOTE is a pitch like C4, F#5, A#3 (sharps only),
 * several pitches joined by + for a chord, or r for a rest. `|` marks bars and is ignored.
 */

export type Instrument = "bell" | "pad" | "organ" | "pluck";

export interface Voice {
  instrument: Instrument;
  notes: string;
  /** Octaves to shift every pitch by. */
  octave?: number;
  /** Relative loudness, 1 = default. */
  gain?: number;
}

export interface Recording {
  /** Path under /public. */
  src: string;
  performer: string;
  license: string;
  licenseUrl?: string;
  /** Commons file page. */
  source: string;
}

export interface Score {
  title: string;
  composer: string;
  recording?: Recording;
  /** Synthesised arrangement, used when there is no recording or it fails to play. */
  bpm?: number;
  voices?: Voice[];
}

const commons = (file: string) => `https://commons.wikimedia.org/wiki/File:${encodeURIComponent(file.replace(/ /g, "_"))}`;
const PD = { license: "Public domain" };
const CC_BY_3 = { license: "CC BY 3.0", licenseUrl: "https://creativecommons.org/licenses/by/3.0/" };
const CC_BY_SA_4 = { license: "CC BY-SA 4.0", licenseUrl: "https://creativecommons.org/licenses/by-sa/4.0/" };

const rep = (s: string, n: number) => Array(n).fill(s).join(" ");

/** Oom-pah bass: each root for `beats`, alternating low and high octave on every half. */
const oompah = (roots: string, beats = 4) =>
  roots
    .split(" ")
    .map((r) => `${r}2:${beats / 2} ${r}3:${beats / 2}`)
    .join(" ");

const JINGLE_BELLS: Score = {
  title: "Jingle Bells",
  composer: "James Lord Pierpont, 1857",
  recording: { src: "/music/jingle-bells.m4a", performer: "Kevin MacLeod", ...CC_BY_3, source: commons("Jingle Bells (Calm) (Kevin MacLeod) (ISRC USUAN1100188).oga") },
  bpm: 168,
  voices: [
    {
      instrument: "bell",
      notes: `
        G4:1 E5:1 D5:1 C5:1 | G4:3 G4:.5 G4:.5 | G4:1 E5:1 D5:1 C5:1 | A4:4 |
        A4:1 F5:1 E5:1 D5:1 | B4:4 | G5:1 G5:1 F5:1 D5:1 | E5:4 |
        G4:1 E5:1 D5:1 C5:1 | G4:4 | G4:1 E5:1 D5:1 C5:1 | A4:3 A4:1 |
        A4:1 F5:1 E5:1 D5:1 | G5:1 G5:1 G5:1 G5:1 | A5:1 G5:1 F5:1 D5:1 | C5:4 |
        E5:1 E5:1 E5:2 | E5:1 E5:1 E5:2 | E5:1 G5:1 C5:1.5 D5:.5 | E5:4 |
        F5:1 F5:1 F5:1.5 F5:.5 | F5:1 E5:1 E5:1 E5:.5 E5:.5 | E5:1 D5:1 D5:1 E5:1 | D5:2 G5:2 |
        E5:1 E5:1 E5:2 | E5:1 E5:1 E5:2 | E5:1 G5:1 C5:1.5 D5:.5 | E5:4 |
        F5:1 F5:1 F5:1.5 F5:.5 | F5:1 E5:1 E5:1 E5:.5 E5:.5 | G5:1 G5:1 F5:1 D5:1 | C5:4`,
    },
    {
      instrument: "pluck",
      gain: 0.7,
      notes: oompah("C C C F F G G C C C C F F G G C C C C C F C D G C C C C F C G C"),
    },
  ],
};

const WE_WISH: Score = {
  title: "We Wish You a Merry Christmas",
  composer: "Traditional English carol",
  recording: { src: "/music/we-wish-you.m4a", performer: "Kevin MacLeod", ...PD, source: commons("We Wish you a Merry Christmas (Kevin MacLeod ) (ISRC USUAN1100369).oga") },
  bpm: 150,
  voices: [
    {
      instrument: "bell",
      octave: 1,
      notes: `
        D4:1 | G4:1 G4:.5 A4:.5 G4:.5 F#4:.5 | E4:1 E4:1 E4:1 | A4:1 A4:.5 B4:.5 A4:.5 G4:.5 | F#4:1 D4:1 D4:1 |
        B4:1 B4:.5 C5:.5 B4:.5 A4:.5 | G4:1 E4:1 D4:.5 D4:.5 | E4:1 A4:1 F#4:1 | G4:2`,
    },
    { instrument: "pluck", gain: 0.7, notes: "r:1 G2:3 C3:3 A2:3 D3:3 B2:3 E3:3 C3:1 D3:2 G2:2" },
  ],
};

const AULD_LANG_SYNE: Score = {
  title: "Auld Lang Syne",
  composer: "Traditional Scottish",
  recording: { src: "/music/auld-lang-syne.m4a", performer: "U.S. Army Band", ...PD, source: commons("Auld Lang Syne - U.S. Army Band.ogg") },
  bpm: 92,
  voices: [
    {
      instrument: "bell",
      octave: 1,
      notes: `
        C4:1 | F4:1.5 F4:.5 F4:1 A4:1 | G4:1.5 F4:.5 G4:1 A4:.5 G4:.5 | F4:1.5 F4:.5 A4:1 C5:1 | D5:3 D5:1 |
        C5:1.5 A4:.5 A4:1 F4:1 | G4:1.5 F4:.5 G4:1 A4:.5 G4:.5 | F4:1.5 D4:.5 D4:1 C4:1 | F4:3 D5:1 |
        C5:1.5 A4:.5 A4:1 F4:1 | G4:1.5 F4:.5 G4:1 D5:1 | C5:1.5 A4:.5 A4:1 C5:1 | D5:3 F5:1 |
        C5:1.5 A4:.5 A4:1 F4:1 | G4:1.5 F4:.5 G4:1 A4:.5 G4:.5 | F4:1.5 D4:.5 D4:1 C4:1 | F4:3`,
    },
    {
      instrument: "pad",
      gain: 0.6,
      notes: "r:1 F2+C3:4 C3+G3:4 F2+C3:4 A#2+F3:4 F2+C3:4 C3+G3:4 D3+A3:4 F2+C3:4 F2+C3:4 C3+G3:4 F2+C3:4 A#2+F3:4 F2+C3:4 C3+G3:4 D3+A3:4 F2+C3:3",
    },
  ],
};

const CANON: Score = {
  title: "Canon in D",
  composer: "Johann Pachelbel, c. 1680",
  recording: { src: "/music/canon.m4a", performer: "Kevin MacLeod", ...CC_BY_3, source: commons("Kevin MacLeod - Canon in D Major.ogg") },
  bpm: 66,
  voices: [
    { instrument: "pad", gain: 0.7, notes: rep("D3:2 A2:2 B2:2 F#2:2 G2:2 D2:2 G2:2 A2:2", 3) },
    {
      instrument: "bell",
      notes: `
        F#5:2 E5:2 D5:2 C#5:2 B4:2 A4:2 B4:2 C#5:2 |
        D5:2 C#5:2 B4:2 A4:2 G4:2 F#4:2 G4:2 E4:2 |
        D4:.5 F#4:.5 A4:.5 G4:.5 F#4:.5 D4:.5 F#4:.5 E4:.5 D4:.5 B3:.5 D4:.5 A4:.5 G4:.5 B4:.5 A4:.5 G4:.5 |
        F#4:.5 D4:.5 E4:.5 C#5:.5 D5:.5 F#5:.5 A5:.5 A4:.5 B4:.5 G4:.5 A4:.5 F#4:.5 D4:.5 D5:.5 D5:1`,
    },
  ],
};

const FUR_ELISE: Score = {
  title: "Für Elise",
  composer: "Ludwig van Beethoven, 1810",
  recording: { src: "/music/fur-elise.m4a", performer: "JMC Han", ...CC_BY_SA_4, source: commons("For Elise (Für Elise) Beethoven JMC Han.ogg") },
  bpm: 70,
  voices: [
    {
      instrument: "bell",
      notes: `
        E5:.25 D#5:.25 | E5:.25 D#5:.25 E5:.25 B4:.25 D5:.25 C5:.25 | A4:.5 r:.25 C4:.25 E4:.25 A4:.25 |
        B4:.5 r:.25 E4:.25 G#4:.25 B4:.25 | C5:.5 r:.25 E4:.25 E5:.25 D#5:.25 |
        E5:.25 D#5:.25 E5:.25 B4:.25 D5:.25 C5:.25 | A4:.5 r:.25 C4:.25 E4:.25 A4:.25 |
        B4:.5 r:.25 E4:.25 C5:.25 B4:.25 | A4:1.5`,
    },
    {
      instrument: "pluck",
      gain: 0.6,
      notes: `
        r:.5 | r:1.5 | A2:.25 E3:.25 A3:.25 r:.75 | E2:.25 E3:.25 G#3:.25 r:.75 | A2:.25 E3:.25 A3:.25 r:.75 |
        r:1.5 | A2:.25 E3:.25 A3:.25 r:.75 | E2:.25 E3:.25 G#3:.25 r:.75 | A2:.25 E3:.25 A3:.25 r:.75`,
    },
  ],
};

const MOONLIGHT: Score = {
  title: "Moonlight Sonata",
  composer: "Ludwig van Beethoven, 1801",
  recording: { src: "/music/moonlight.m4a", performer: "Musopen", ...PD, source: commons("Ludwig van Beethoven - sonata no. 14 in c sharp minor 'moonlight', op. 27 no. 2 - i. adagio sostenuto.ogg") },
  bpm: 56,
  voices: [
    {
      instrument: "pad",
      gain: 0.8,
      notes: "C#2+C#3:4 B1+B2:4 A1+A2:2 F#1+F#2:2 G#1+G#2:4 C#2+C#3:4 C#2+C#3:4",
    },
    {
      instrument: "bell",
      gain: 0.75,
      notes: [
        rep("G#3:.333 C#4:.333 E4:.334", 8),
        rep("A3:.333 C#4:.333 E4:.334", 2),
        rep("A3:.333 D4:.333 F#4:.334", 2),
        rep("G#3:.333 C4:.333 F#4:.334", 2),
        "G#3:.333 C#4:.333 E4:.334",
        "F#3:.333 C4:.333 D#4:.334",
        rep("E3:.333 G#3:.333 C#4:.334", 4),
        "r:4",
      ].join(" "),
    },
  ],
};

const TOCCATA: Score = {
  title: "Toccata and Fugue in D minor",
  composer: "Johann Sebastian Bach",
  recording: { src: "/music/toccata.m4a", performer: "Kevin MacLeod", ...CC_BY_3, source: commons("Kevin MacLeod - J S Bach Toccata and Fugue in D Minor.ogg") },
  bpm: 60,
  voices: [0, -1].map((octave) => ({
    instrument: "organ" as const,
    octave,
    gain: octave === 0 ? 1 : 0.8,
    notes: `
      A5:.125 G5:.125 A5:1.5 r:.5 G5:.125 F5:.125 E5:.125 D5:.125 C#5:.75 D5:1.5 r:1 |
      A4:.125 G4:.125 A4:1.5 r:.5 E4:.25 F4:.25 C#4:.25 D4:1.5 r:1 |
      A3:.125 G3:.125 A3:1.5 r:.5 G3:.125 F3:.125 E3:.125 D3:.125 C#3:.75 D3:2.5 r:1.5`,
  })),
};

const MOUNTAIN_KING: Score = {
  title: "In the Hall of the Mountain King",
  composer: "Edvard Grieg, 1875",
  recording: { src: "/music/mountain-king.m4a", performer: "Musopen Symphony Orchestra", ...PD, source: commons("Musopen - In the Hall Of The Mountain King.ogg") },
  bpm: 132,
  voices: [
    {
      instrument: "pluck",
      octave: 1,
      notes: rep(
        `B3:.5 C#4:.5 D4:.5 E4:.5 F#4:.5 D4:.5 F#4:1 | F4:.5 C#4:.5 F4:1 E4:.5 C4:.5 E4:1 |
         B3:.5 C#4:.5 D4:.5 E4:.5 F#4:.5 D4:.5 F#4:.5 B4:.5 | A4:.5 F#4:.5 D4:.5 F#4:.5 A4:2`,
        2,
      ),
    },
    { instrument: "pluck", gain: 0.7, notes: rep("B2:.5 r:.5 F#2:.5 r:.5", 16) },
  ],
};

const SPRING: Score = {
  title: "The Four Seasons — Spring",
  composer: "Antonio Vivaldi, 1725",
  recording: { src: "/music/spring.m4a", performer: "John Harrison, violin", ...CC_BY_SA_4, source: commons("Vivaldi - Four Seasons 1 Spring mvt 1 Allegro - John Harrison violin.oga") },
  bpm: 100,
  voices: [
    {
      instrument: "bell",
      notes: `
        E5:1 | G#5:1 G#5:1 G#5:1 F#5:.5 E5:.5 | B5:3 B5:.5 A5:.5 | G#5:1 G#5:1 G#5:1 F#5:.5 E5:.5 |
        B5:3 B5:.5 A5:.5 | G#5:1 A5:.5 B5:.5 A5:1 G#5:1 | F#5:3`,
    },
    { instrument: "pad", gain: 0.6, notes: "r:1 E3+B3:4 E3+B3:4 E3+B3:4 E3+B3:4 E3+B3:2 B2+F#3:2 B2+F#3:3" },
  ],
};

const MORNING_MOOD: Score = {
  title: "Peer Gynt — Morning Mood",
  composer: "Edvard Grieg, 1875",
  recording: { src: "/music/morning-mood.m4a", performer: "Musopen Symphony Orchestra", ...PD, source: commons("Musopen - Morning.ogg") },
  bpm: 66,
  voices: [
    {
      instrument: "bell",
      notes: `
        E5:.75 C#5:.25 B4:.5 A4:.5 B4:.5 C#5:.5 | E5:.75 C#5:.25 B4:.5 A4:.5 B4:.5 C#5:.5 |
        B4:.5 C#5:.5 E5:.5 C#5:.5 E5:.5 F#5:.5 | C#5:.5 F#5:.5 E5:.5 C#5:.5 B4:.5 A4:.5 | A4:3`,
    },
    { instrument: "pad", gain: 0.6, notes: "A2+E3+A3:3 A2+E3+C#4:3 A2+E3+A3:3 F#2+C#3+A3:3 A2+E3+A3:3" },
  ],
};

const VIVALDI_AUTUMN: Score = {
  title: "The Four Seasons — Autumn",
  composer: "Antonio Vivaldi, 1725",
  recording: { src: "/music/autumn.m4a", performer: "John Harrison, violin", ...CC_BY_SA_4, source: commons("Vivaldi - Four Seasons 3 Autumn mvt 1 Allegro - John Harrison violin.oga") },
};

const VIVALDI_WINTER: Score = {
  title: "The Four Seasons — Winter",
  composer: "Antonio Vivaldi, 1725",
  recording: { src: "/music/winter.m4a", performer: "John Harrison, violin", ...CC_BY_SA_4, source: commons("Vivaldi - Four Seasons 4 Winter mvt 1 Allegro non molto - John Harrison violin.oga") },
};

/** "Composer · performer" for display. */
export const credit = (s: Score) => (s.recording ? `${s.composer} · ${s.recording.performer}` : s.composer);

/** Playlist per theme, played in turn. */
export const PLAYLIST: Record<ThemeId, Score[]> = {
  christmas: [WE_WISH, JINGLE_BELLS],
  winter: [VIVALDI_WINTER],
  newyear: [AULD_LANG_SYNE],
  // Well-known Vietnamese Tết songs are still under copyright; Tết is the spring festival.
  tet: [SPRING],
  valentine: [CANON],
  womensday: [FUR_ELISE],
  midautumn: [MOONLIGHT],
  halloween: [TOCCATA, MOUNTAIN_KING],
  spring: [SPRING],
  summer: [MORNING_MOOD],
  autumn: [VIVALDI_AUTUMN],
};
