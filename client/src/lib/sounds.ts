// Procedural sound effects using Web Audio API.
// No external files needed — sounds are synthesized on demand.
//
// Design notes:
// - Kid-friendly: cheerful, soft, short.
// - All sounds peak around -6dB so they don't startle anyone.
// - Mute state is persisted in localStorage.

const STORAGE_KEY = "spacetag.muted";

class SoundManager {
  private ctx: AudioContext | null = null;
  private muted = false;
  private masterGain: GainNode | null = null;

  constructor() {
    if (typeof window !== "undefined") {
      this.muted = localStorage.getItem(STORAGE_KEY) === "1";
    }
  }

  /** Lazily initialize the AudioContext (must happen on a user gesture) */
  private ensureCtx(): AudioContext | null {
    if (typeof window === "undefined") return null;
    if (!this.ctx) {
      try {
        const Ctx =
          window.AudioContext ||
          (window as unknown as { webkitAudioContext: typeof AudioContext })
            .webkitAudioContext;
        this.ctx = new Ctx();
        this.masterGain = this.ctx.createGain();
        this.masterGain.gain.value = 0.5;
        this.masterGain.connect(this.ctx.destination);
      } catch {
        return null;
      }
    }
    // Some browsers suspend the context until a user gesture
    if (this.ctx.state === "suspended") {
      this.ctx.resume();
    }
    return this.ctx;
  }

  isMuted() {
    return this.muted;
  }

  toggleMute() {
    this.muted = !this.muted;
    if (typeof window !== "undefined") {
      localStorage.setItem(STORAGE_KEY, this.muted ? "1" : "0");
    }
    return this.muted;
  }

  /** Play a single tone with an envelope */
  private tone(opts: {
    freq: number;
    duration: number;
    type?: OscillatorType;
    volume?: number;
    delay?: number;
    sweepTo?: number; // optional pitch slide target
  }) {
    if (this.muted) return;
    const ctx = this.ensureCtx();
    if (!ctx || !this.masterGain) return;

    const {
      freq,
      duration,
      type = "sine",
      volume = 0.3,
      delay = 0,
      sweepTo,
    } = opts;

    const start = ctx.currentTime + delay;
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    osc.type = type;
    osc.frequency.setValueAtTime(freq, start);
    if (sweepTo !== undefined) {
      osc.frequency.exponentialRampToValueAtTime(sweepTo, start + duration);
    }

    // Quick attack + exponential decay envelope
    gain.gain.setValueAtTime(0.0001, start);
    gain.gain.exponentialRampToValueAtTime(volume, start + 0.005);
    gain.gain.exponentialRampToValueAtTime(0.0001, start + duration);

    osc.connect(gain);
    gain.connect(this.masterGain);
    osc.start(start);
    osc.stop(start + duration + 0.05);
  }

  /** Short noise burst (used for whooshes, sparkles) */
  private noise(opts: {
    duration: number;
    volume?: number;
    delay?: number;
    filterFreq?: number;
  }) {
    if (this.muted) return;
    const ctx = this.ensureCtx();
    if (!ctx || !this.masterGain) return;

    const { duration, volume = 0.15, delay = 0, filterFreq = 4000 } = opts;
    const start = ctx.currentTime + delay;

    const bufferSize = Math.floor(ctx.sampleRate * duration);
    const buffer = ctx.createBuffer(1, bufferSize, ctx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }

    const src = ctx.createBufferSource();
    src.buffer = buffer;

    const filter = ctx.createBiquadFilter();
    filter.type = "bandpass";
    filter.frequency.value = filterFreq;
    filter.Q.value = 1;

    const gain = ctx.createGain();
    gain.gain.setValueAtTime(0.0001, start);
    gain.gain.exponentialRampToValueAtTime(volume, start + 0.005);
    gain.gain.exponentialRampToValueAtTime(0.0001, start + duration);

    src.connect(filter);
    filter.connect(gain);
    gain.connect(this.masterGain);
    src.start(start);
    src.stop(start + duration + 0.05);
  }

  // ---- Public sound effects ----

  /** Soft UI click */
  click() {
    this.tone({ freq: 600, duration: 0.06, type: "triangle", volume: 0.2 });
  }

  /** Cheerful task-complete chime (ascending arpeggio) */
  taskComplete() {
    this.tone({ freq: 523, duration: 0.12, type: "sine", volume: 0.3, delay: 0 });
    this.tone({ freq: 659, duration: 0.12, type: "sine", volume: 0.3, delay: 0.1 });
    this.tone({ freq: 784, duration: 0.2, type: "sine", volume: 0.35, delay: 0.2 });
  }

  /** Magical "freeze" poof (descending sine + noise sparkle) */
  freeze() {
    this.tone({
      freq: 1200,
      sweepTo: 200,
      duration: 0.4,
      type: "triangle",
      volume: 0.25,
    });
    this.noise({ duration: 0.3, volume: 0.1, filterFreq: 6000 });
  }

  /** Doorbell-style alert when a body is reported */
  reportBody() {
    this.tone({ freq: 523, duration: 0.18, type: "sine", volume: 0.35 });
    this.tone({ freq: 392, duration: 0.25, type: "sine", volume: 0.35, delay: 0.18 });
  }

  /** Meeting bell (two ringing chimes) */
  meetingStart() {
    this.tone({ freq: 880, duration: 0.4, type: "triangle", volume: 0.3 });
    this.tone({ freq: 880, duration: 0.4, type: "triangle", volume: 0.3, delay: 0.25 });
  }

  /** Vote stamp click */
  vote() {
    this.tone({ freq: 200, duration: 0.08, type: "square", volume: 0.25 });
    this.tone({ freq: 150, duration: 0.05, type: "square", volume: 0.2, delay: 0.04 });
  }

  /** Whoosh for a player being "sent home" after vote */
  ejection() {
    this.noise({ duration: 0.6, volume: 0.18, filterFreq: 1500 });
  }

  /** Win fanfare — cheerful ascending major chord */
  win() {
    this.tone({ freq: 523, duration: 0.2, type: "triangle", volume: 0.35 });
    this.tone({ freq: 659, duration: 0.2, type: "triangle", volume: 0.35, delay: 0.15 });
    this.tone({ freq: 784, duration: 0.2, type: "triangle", volume: 0.35, delay: 0.3 });
    this.tone({ freq: 1047, duration: 0.4, type: "triangle", volume: 0.4, delay: 0.45 });
  }

  /** Gentle "aww" descending tune */
  lose() {
    this.tone({ freq: 523, duration: 0.25, type: "sine", volume: 0.3 });
    this.tone({ freq: 466, duration: 0.25, type: "sine", volume: 0.3, delay: 0.2 });
    this.tone({ freq: 392, duration: 0.5, type: "sine", volume: 0.3, delay: 0.4 });
  }

  /** Tagger cooldown ready (subtle ping) */
  tagReady() {
    this.tone({ freq: 880, duration: 0.08, type: "sine", volume: 0.2 });
  }

  /** Tag/freeze sound (the tagger's perspective) */
  tag() {
    this.tone({ freq: 200, duration: 0.15, type: "sawtooth", volume: 0.25 });
    this.noise({ duration: 0.2, volume: 0.12, filterFreq: 2000 });
  }
}

export const sounds = new SoundManager();
