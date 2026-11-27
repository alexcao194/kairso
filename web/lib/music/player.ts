import type { Instrument, Score } from "./scores";

/**
 * A tiny Web Audio synth that plays the festive scores: soft music-box bells, a warm pad, a
 * plucked bass and a reed organ. Notes are scheduled a little ahead on the audio clock, voices
 * loop together, and the playlist moves on after each piece. Quiet by design (background music).
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
const GAP_BEATS = 2; // silence between pieces

export class FestivePlayer {
  private ctx: AudioContext | null = null;
  private master: GainNode | null = null;
  private timer = 0;
  private playlist: Score[] = [];
  private index = 0;
  private queue: { at: number; dur: number; freqs: number[]; inst: Instrument; gain: number }[] = [];
  private nextStart = 0;
  private live = new Set<AudioScheduledSourceNode>();
  onTrack?: (score: Score) => void;

  get playing() {
    return this.timer !== 0;
  }

  async play(playlist: Score[], volume = 0.14) {
    this.stop(0);
    if (!playlist.length) return;
    this.ctx ??= new AudioContext();
    await this.ctx.resume();
    const ctx = this.ctx;
    this.master = ctx.createGain();
    this.master.gain.setValueAtTime(0, ctx.currentTime);
    this.master.gain.linearRampToValueAtTime(volume, ctx.currentTime + 1.2);
    // Gentle low-pass keeps the synth soft.
    const tone = ctx.createBiquadFilter();
    tone.type = "lowpass";
    tone.frequency.value = 3200;
    this.master.connect(tone).connect(ctx.destination);
    this.playlist = playlist;
    this.index = 0;
    this.nextStart = ctx.currentTime + 0.15;
    this.enqueue();
    this.timer = window.setInterval(() => this.pump(), TICK);
  }

  /** Fade out and stop. */
  stop(fade = 0.6) {
    window.clearInterval(this.timer);
    this.timer = 0;
    this.queue = [];
    const ctx = this.ctx, master = this.master;
    this.master = null;
    if (!ctx || !master) return;
    const now = ctx.currentTime;
    master.gain.cancelScheduledValues(now);
    master.gain.setValueAtTime(master.gain.value, now);
    master.gain.linearRampToValueAtTime(0, now + fade);
    const nodes = [...this.live];
    this.live.clear();
    window.setTimeout(() => {
      for (const n of nodes) {
        try { n.stop(); } catch { /* already stopped */ }
      }
      master.disconnect();
    }, fade * 1000 + 50);
  }

  /** Queue the current piece starting at nextStart, then advance the playlist. */
  private enqueue() {
    const score = this.playlist[this.index % this.playlist.length];
    this.index++;
    const spb = 60 / score.bpm;
    let longest = 0;
    for (const v of score.voices) {
      const { events, length } = parseVoice(v.notes, v.octave ?? 0);
      longest = Math.max(longest, length);
      for (const e of events) {
        this.queue.push({ at: this.nextStart + e.time * spb, dur: e.dur * spb, freqs: e.freqs, inst: v.instrument, gain: v.gain ?? 1 });
      }
    }
    this.queue.sort((a, b) => a.at - b.at);
    this.onTrack?.(score);
    this.nextStart += (longest + GAP_BEATS) * spb;
  }

  private pump() {
    const ctx = this.ctx;
    if (!ctx || !this.master) return;
    const horizon = ctx.currentTime + LOOKAHEAD;
    while (this.queue.length && this.queue[0].at < horizon) {
      const n = this.queue.shift()!;
      for (const f of n.freqs) this.voice(n.inst, f, Math.max(n.at, ctx.currentTime), n.dur, n.gain / Math.sqrt(n.freqs.length));
    }
    if (!this.queue.length && this.nextStart - ctx.currentTime < LOOKAHEAD * 2) this.enqueue();
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
