// Procedural sound effects using Web Audio API.
// No external files needed — sounds are synthesized on demand.
//
// Design notes:
// - Kid-friendly: cheerful, soft, short.
// - All sounds peak around -6dB so they don't startle anyone.
// - Mute state is persisted in localStorage.

const STORAGE_KEY = "spacetag.muted";
const MUSIC_KEY = "spacetag.musicMuted";

// Gentle C-major pentatonic arpeggio, looped. Low enough to sit under SFX.
// 8 notes, played at ~80 BPM (0.75s each) = 6s loop.
const MUSIC_NOTES = [
  261.63, // C4
  329.63, // E4
  392.0, // G4
  523.25, // C5
  440.0, // A4
  392.0, // G4
  329.63, // E4
  293.66, // D4
];
const NOTE_INTERVAL = 0.75; // seconds per note
const NOTE_DURATION = 1.4; // longer than interval → overlap for smooth sustain
const MUSIC_LOOKAHEAD = 0.2; // how far ahead we schedule

class SoundManager {
  private ctx: AudioContext | null = null;
  private muted = false;
  private musicMuted = false;
  private masterGain: GainNode | null = null;
  private musicGain: GainNode | null = null;
  private unlocked = false;

  // Music scheduler state
  private musicTimer: number | null = null;
  private nextNoteTime = 0;
  private noteIndex = 0;

  constructor() {
    if (typeof window !== "undefined") {
      this.muted = localStorage.getItem(STORAGE_KEY) === "1";
      this.musicMuted = localStorage.getItem(MUSIC_KEY) === "1";
      this.installUnlockHandler();
    }
  }

  /**
   * Attach listeners that create and resume the AudioContext on any user
   * interaction. Required because iOS Safari and Chrome refuse to create or
   * resume an AudioContext outside a user gesture, and many of our sounds
   * are triggered by WebSocket messages (freeze, meetingStart, etc.) which
   * are NOT user gestures.
   *
   * Subtlety: `ctx.resume()` is async. The listener stays attached until
   * the context reports `state === 'running'`, so a failed first attempt
   * (e.g. iPad Safari refusing a `pointerdown` for some reason) doesn't
   * leave us deaf for the whole session. Once running, we start music and
   * remove the listeners.
   */
  private installUnlockHandler() {
    const EVENTS = ["pointerdown", "touchstart", "touchend", "click", "keydown"];
    const cleanup = () => {
      for (const ev of EVENTS) {
        window.removeEventListener(ev, unlock);
      }
    };
    const unlock = () => {
      this.unlock();
      // Only tear down once the context actually transitions to running.
      // If resume() hasn't resolved yet, wait for it here via statechange.
      if (this.ctx && this.ctx.state === "running") {
        cleanup();
        if (!this.musicTimer) this.startMusic();
      }
    };
    for (const ev of EVENTS) {
      window.addEventListener(ev, unlock);
    }
  }

  /**
   * Public unlock — safe to call from any confirmed user-gesture handler.
   * Creates the AudioContext (if needed), resumes it, and plays a silent
   * buffer to fully unlock iOS audio. Idempotent.
   */
  unlock() {
    const ctx = this.ensureCtx();
    if (!ctx) return;
    // Play a one-sample silent buffer to fully unlock iOS audio. Must be
    // called synchronously inside the gesture callback.
    try {
      const buf = ctx.createBuffer(1, 1, 22050);
      const src = ctx.createBufferSource();
      src.buffer = buf;
      src.connect(ctx.destination);
      src.start(0);
    } catch {
      // ignore
    }
    // Listen for the state transition to fire music exactly once.
    if (!this.unlocked) {
      const onState = () => {
        if (ctx.state === "running") {
          this.unlocked = true;
          if (!this.musicTimer) this.startMusic();
          ctx.removeEventListener("statechange", onState);
        }
      };
      ctx.addEventListener("statechange", onState);
      // If it's already running (desktop Chrome with autoplay allowed),
      // fire immediately.
      if (ctx.state === "running") onState();
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
        // Separate gain bus for music so the music toggle is independent
        // from the SFX mute toggle.
        this.musicGain = this.ctx.createGain();
        this.musicGain.gain.value = this.musicMuted ? 0 : 0.12;
        this.musicGain.connect(this.masterGain);
      } catch {
        return null;
      }
    }
    // Some browsers suspend the context until a user gesture. `resume()`
    // returns a promise but we intentionally don't await — audio will start
    // playing on the next frame once the context wakes up.
    if (this.ctx.state === "suspended") {
      this.ctx.resume().catch(() => {});
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

  isMusicMuted() {
    return this.musicMuted;
  }

  toggleMusic() {
    this.musicMuted = !this.musicMuted;
    if (typeof window !== "undefined") {
      localStorage.setItem(MUSIC_KEY, this.musicMuted ? "1" : "0");
    }
    // Smoothly fade the music gain rather than clicking on/off
    const ctx = this.ctx;
    if (ctx && this.musicGain) {
      const target = this.musicMuted ? 0 : 0.12;
      this.musicGain.gain.cancelScheduledValues(ctx.currentTime);
      this.musicGain.gain.linearRampToValueAtTime(target, ctx.currentTime + 0.3);
    }
    // Make sure the scheduler is running so unmuting actually produces sound
    if (!this.musicMuted && !this.musicTimer) {
      this.startMusic();
    }
    return this.musicMuted;
  }

  /** Start the background music scheduler. Safe to call multiple times. */
  startMusic() {
    if (this.musicTimer !== null) return;
    const ctx = this.ensureCtx();
    if (!ctx) return;
    this.nextNoteTime = ctx.currentTime + 0.1;
    this.noteIndex = 0;
    this.musicTimer = window.setInterval(() => this.scheduleMusic(), 100);
  }

  /** Stop the background music scheduler. */
  stopMusic() {
    if (this.musicTimer !== null) {
      clearInterval(this.musicTimer);
      this.musicTimer = null;
    }
  }

  /**
   * Look-ahead scheduler: schedules notes a short time in advance so timing
   * is sample-accurate even if the JS timer is jittery.
   */
  private scheduleMusic() {
    const ctx = this.ctx;
    if (!ctx || !this.musicGain) return;
    while (this.nextNoteTime < ctx.currentTime + MUSIC_LOOKAHEAD) {
      this.scheduleNote(MUSIC_NOTES[this.noteIndex], this.nextNoteTime);
      this.nextNoteTime += NOTE_INTERVAL;
      this.noteIndex = (this.noteIndex + 1) % MUSIC_NOTES.length;
    }
  }

  /** Schedule a single music note (triangle wave + sub-octave for warmth). */
  private scheduleNote(freq: number, startTime: number) {
    const ctx = this.ctx;
    if (!ctx || !this.musicGain) return;

    const makeVoice = (f: number, type: OscillatorType, vol: number) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = type;
      osc.frequency.setValueAtTime(f, startTime);
      // Slow attack + slow release for a soft pad feel
      gain.gain.setValueAtTime(0.0001, startTime);
      gain.gain.exponentialRampToValueAtTime(vol, startTime + 0.15);
      gain.gain.exponentialRampToValueAtTime(
        0.0001,
        startTime + NOTE_DURATION
      );
      osc.connect(gain);
      gain.connect(this.musicGain!);
      osc.start(startTime);
      osc.stop(startTime + NOTE_DURATION + 0.05);
    };

    makeVoice(freq, "triangle", 0.5);
    makeVoice(freq / 2, "sine", 0.3); // sub-octave for warmth
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
