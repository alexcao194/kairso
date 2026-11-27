import type { Instrument, Score } from "./scores";

/**
 * Festive music player. Pieces with a recording play through one shared <audio> element (one
 * element, so Safari keeps it unlocked after the first user gesture); pieces without one, or
 * whose file fails, are synthesised with Web Audio: soft music-box bells, a warm pad, a plucked
 * bass and a reed organ, scheduled a little ahead on the audio clock. The playlist moves on after
 * each piece. `level` is the viewer's 0..1 volume; both paths stay background-quiet.
 */

const NOTE_INDEX: Record<string, number> = { C: 0, "C#": 1, D: 2, "D#": 3, E: 4, F: 5, "F#": 6, G: 7, "G#": 8, A: 9, "A#": 10, B: 11 };

interface Ev { time: number; dur: number; freqs: number[] }

function freq(pitch: string, shift: number): number {
  const m = /^([A-G]#?)(-?\d)$/.exec(pitch);
  if (!m) throw new Error(`bad pitch ${pitch}`);
  const midi = (Number(m[2]) + shift + 1) * 12 + NOTE_INDEX[m[1]];
  return 440 * 2 ** ((midi - 69) / 12);
}

/** Parse a voice into events in beats; returns the events and the voice's length. */
export function parseVoice(notes: string, octave = 0): { events: Ev[]; length: number } {
  const events: Ev[] = [];
  let t = 0;
  for (const tok of notes.split(/\s+/)) {
    if (!tok || tok === "|") continue;
    const i = tok.lastIndexOf(":");
    const beats = Number(tok.slice(i + 1));
    const pitch = tok.slice(0, i);
    if (!Number.isFinite(beats) || beats <= 0) throw new Error(`bad token ${tok}`);
    if (pitch !== "r") events.push({ time: t, dur: beats, freqs: pitch.split("+").map((p) => freq(p, octave)) });
    t += beats;
  }
  return { events, length: t };
}

const LOOKAHEAD = 0.25; // seconds scheduled ahead
const TICK = 60; // ms between scheduling passes
const GAP_BEATS = 2; // silence after a synthesised piece
const GAP_MS = 1500; // silence after a recording
const SYNTH_GAIN = 0.25; // master gain of the synth at full volume
const RECORDING_GAIN = 0.7; // <audio> volume at full volume

type Queued = { at: number; dur: number; freqs: number[]; inst: Instrument; gain: number };

export class FestivePlayer {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private audio: HTMLAudioElement | null = null;
  private timer = 0;
  private nextTimer = 0;
  private fadeTimer = 0;
  private queue: Queued[] = [];
  private live = new Set<AudioScheduledSourceNode>();
  private playlist: Score[] = [];
  private index = 0;
  private level = 0.6;
  /** Bumped on every play/stop so callbacks of an older run do nothing. */
  private run = 0;
  onTrack?: (score: Score) => void;
  /** Called when nothing in the playlist can play. */
  onFail?: () => void;

  /** Change the loudness (0..1) while playing. */
  setVolume(level: number) {
    this.level = level;
    if (this.audio && !this.fadeTimer) this.audio.volume = level * RECORDING_GAIN;
    if (this.ctx && this.master) this.master.gain.setTargetAtTime(level * SYNTH_GAIN, this.ctx.currentTime, 0.1);
  }

  /** Start the playlist. Call from a user gesture: the first audio.play() must happen inside it. */
  async play(playlist: Score[], level = this.level) {
    this.stop(0);
    this.level = level;
    this.playlist = playlist;
    this.index = 0;
    if (!playlist.length) return;
    await this.next(this.run, 0);
  }

  /** Fade out and stop. */
  stop(fade = 0.6) {
    this.run++;
    window.clearTimeout(this.nextTimer);
    this.stopSynth(fade);
    this.stopRecording(fade);
  }

  private async next(run: number, failures: number) {
    if (run !== this.run) return;
    if (failures >= this.playlist.length) {
      this.onFail?.();
      return;
    }
    const score = this.playlist[this.index++ % this.playlist.length];
    this.onTrack?.(score);
    if (score.recording) {
      try {
        await this.playRecording(score, run);
        return;
      } catch {
        if (run !== this.run) return;
      }
    }
    if (score.voices?.length) {
      await this.playSynth(score, run);
      return;
    }
    await this.next(run, failures + 1);
  }

  // ---------- recordings ----------

  private async playRecording(score: Score, run: number) {
    const a = (this.audio ??= new Audio());
    window.clearInterval(this.fadeTimer);
    this.fadeTimer = 0;
    a.preload = "auto";
    a.src = score.recording!.src;
    a.volume = this.level * RECORDING_GAIN;
    a.onended = () => {
      if (run === this.run) this.nextTimer = window.setTimeout(() => void this.next(run, 0), GAP_MS);
    };
    a.onerror = () => {
      if (run === this.run && score.voices?.length) void this.playSynth(score, run);
      else if (run === this.run) void this.next(run, 1);
    };
    await a.play();
  }

  private stopRecording(fade: number) {
    const a = this.audio;
    if (!a || a.paused) return;
    a.onended = null;
    a.onerror = null;
    window.clearInterval(this.fadeTimer);
    if (fade <= 0) {
      a.pause();
      return;
    }
    const start = a.volume;
    const steps = Math.max(1, Math.round((fade * 1000) / 40));
    let i = 0;
    this.fadeTimer = window.setInterval(() => {
      i++;
      a.volume = Math.max(0, start * (1 - i / steps));
      if (i >= steps) {
        window.clearInterval(this.fadeTimer);
        this.fadeTimer = 0;
        a.pause();
      }
    }, 40);
  }

  // ---------- synthesiser ----------

  private async playSynth(score: Score, run: number) {
    this.ctx ??= new AudioContext();
    const ctx = this.ctx;
    await ctx.resume();
    if (run !== this.run) return;
    if (!this.master) {
      this.master = ctx.createGain();
      this.master.gain.setValueAtTime(0, ctx.currentTime);
      this.master.gain.linearRampToValueAtTime(this.level * SYNTH_GAIN, ctx.currentTime + 1.2);
      // Gentle low-pass keeps the synth soft.
      const tone = ctx.createBiquadFilter();
      tone.type = "lowpass";
      tone.frequency.value = 3200;
      this.master.connect(tone).connect(ctx.destination);
    }
    const spb = 60 / (score.bpm ?? 90);
    const start = ctx.currentTime + 0.15;
    let longest = 0;
    for (const v of score.voices ?? []) {
      const { events, length } = parseVoice(v.notes, v.octave ?? 0);
      longest = Math.max(longest, length);
      for (const e of events) this.queue.push({ at: start + e.time * spb, dur: e.dur * spb, freqs: e.freqs, inst: v.instrument, gain: v.gain ?? 1 });
    }
    this.queue.sort((a, b) => a.at - b.at);
    window.clearInterval(this.timer);
    this.timer = window.setInterval(() => this.pump(), TICK);
    const ms = ((longest + GAP_BEATS) * spb + 0.15) * 1000;
    this.nextTimer = window.setTimeout(() => void this.next(run, 0), ms);
  }

  private pump() {
    const ctx = this.ctx;
    if (!ctx || !this.master) return;
    const horizon = ctx.currentTime + LOOKAHEAD;
    while (this.queue.length && this.queue[0].at < horizon) {
      const n = this.queue.shift()!;
      for (const f of n.freqs) this.voice(n.inst, f, Math.max(n.at, ctx.currentTime), n.dur, n.gain / Math.sqrt(n.freqs.length));
    }
    if (!this.queue.length) {
      window.clearInterval(this.timer);
      this.timer = 0;
    }
  }

  private stopSynth(fade: number) {
    window.clearInterval(this.timer);
    this.timer = 0;
    this.queue = [];
    const ctx = this.ctx, master = this.master;
    this.master = null;
    if (!ctx || !master) return;
    const now = ctx.currentTime;
    master.gain.cancelScheduledValues(now);
    master.gain.setValueAtTime(master.gain.value, now);
    master.gain.linearRampToValueAtTime(0, now + Math.max(fade, 0.01));
    const nodes = [...this.live];
    this.live.clear();
    window.setTimeout(() => {
      for (const n of nodes) {
        try { n.stop(); } catch { /* already stopped */ }
      }
      master.disconnect();
    }, fade * 1000 + 50);
  }

  private voice(inst: Instrument, f: number, at: number, dur: number, level: number) {
    const ctx = this.ctx!;
    const out = ctx.createGain();
    out.connect(this.master!);
    const env = out.gain;
    const partials: [OscillatorType, number, number][] = [];
    let end: number;
    switch (inst) {
      case "bell": {
        // Music box: sine with a bright overtone, quick attack, long natural decay.
        partials.push(["sine", 1, 1], ["sine", 2, 0.28], ["sine", 3.01, 0.08]);
        const tail = Math.max(dur, 0.6) + 0.9;
        env.setValueAtTime(0, at);
        env.linearRampToValueAtTime(0.5 * level, at + 0.006);
        env.exponentialRampToValueAtTime(0.001, at + tail);
        end = at + tail;
        break;
      }
      case "pluck": {
        partials.push(["triangle", 1, 1], ["sine", 2, 0.2]);
        const tail = Math.min(dur, 0.5) + 0.35;
        env.setValueAtTime(0, at);
        env.linearRampToValueAtTime(0.5 * level, at + 0.008);
        env.exponentialRampToValueAtTime(0.001, at + tail);
        end = at + tail;
        break;
      }
      case "pad": {
        partials.push(["triangle", 1, 1], ["sine", 2, 0.15], ["sine", 1.003, 0.4]);
        env.setValueAtTime(0, at);
        env.linearRampToValueAtTime(0.22 * level, at + 0.12);
        env.setValueAtTime(0.22 * level, at + Math.max(dur - 0.1, 0.12));
        env.linearRampToValueAtTime(0.0001, at + dur + 0.4);
        end = at + dur + 0.45;
        break;
      }
      case "organ": {
        partials.push(["square", 1, 0.35], ["sine", 2, 0.5], ["sine", 4, 0.18], ["sine", 0.5, 0.35]);
        env.setValueAtTime(0, at);
        env.linearRampToValueAtTime(0.24 * level, at + 0.03);
        env.setValueAtTime(0.24 * level, at + Math.max(dur - 0.05, 0.03));
        env.linearRampToValueAtTime(0.0001, at + dur + 0.25);
        end = at + dur + 0.3;
        break;
      }
    }
    let left = partials.length;
    for (const [type, mult, g] of partials) {
      const osc = ctx.createOscillator();
      osc.type = type;
      osc.frequency.value = f * mult;
      const pg = ctx.createGain();
      pg.gain.value = g;
      osc.connect(pg).connect(out);
      osc.start(at);
      osc.stop(end);
      this.live.add(osc);
      osc.onended = () => {
        this.live.delete(osc);
        pg.disconnect();
        if (--left === 0) out.disconnect();
      };
    }
  }
}
