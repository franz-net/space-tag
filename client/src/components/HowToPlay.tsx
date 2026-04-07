"use client";

import { useState } from "react";

interface Props {
  onClose: () => void;
}

interface Slide {
  emoji: string;
  title: string;
  body: string;
  bgColor: string;
}

const SLIDES: Slide[] = [
  {
    emoji: "🚀",
    title: "Welcome to SpaceTag!",
    body: "Explore the spaceship with your friends. One of you is secretly the Tagger — find them before they freeze everyone!",
    bgColor: "from-blue-900 to-purple-900",
  },
  {
    emoji: "🕹️",
    title: "How to move",
    body: "On a computer, use the arrow keys or W A S D. On a phone, drag the joystick at the bottom-left.",
    bgColor: "from-indigo-900 to-blue-900",
  },
  {
    emoji: "⭐",
    title: "Crewmate? Do tasks!",
    body: "Look at your task list (top-right). Walk to a yellow glowing station and tap the USE button to solve a quick puzzle.",
    bgColor: "from-emerald-900 to-teal-900",
  },
  {
    emoji: "❄️",
    title: "Tagger? Freeze friends!",
    body: "Pretend to do tasks. When you're alone with someone, tap the TAG button to freeze them. Don't get caught!",
    bgColor: "from-red-900 to-orange-900",
  },
  {
    emoji: "🚨",
    title: "Find a frozen friend?",
    body: "Walk up to them and tap REPORT. Everyone gathers in the Cafeteria for a meeting.",
    bgColor: "from-yellow-900 to-amber-900",
  },
  {
    emoji: "🗳️",
    title: "Vote together!",
    body: "Tap icons to share what you saw. Then tap a player's face to vote them out — or tap Skip if you're not sure.",
    bgColor: "from-purple-900 to-pink-900",
  },
  {
    emoji: "👻",
    title: "Frozen? You become a ghost!",
    body: "Float anywhere — no walls, no fog. Crewmate ghosts can still finish tasks to help the team win!",
    bgColor: "from-cyan-900 to-blue-900",
  },
  {
    emoji: "🏆",
    title: "How to win",
    body: "Crew wins by doing all tasks OR voting out the Tagger. Tagger wins by freezing enough Crewmates. Have fun!",
    bgColor: "from-green-900 to-emerald-900",
  },
];

export default function HowToPlay({ onClose }: Props) {
  const [index, setIndex] = useState(0);
  const slide = SLIDES[index];
  const isFirst = index === 0;
  const isLast = index === SLIDES.length - 1;

  const next = () => {
    if (isLast) onClose();
    else setIndex(index + 1);
  };

  const prev = () => {
    if (!isFirst) setIndex(index - 1);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur p-4">
      <div
        className={`relative w-full max-w-md rounded-3xl p-8 bg-gradient-to-br ${slide.bgColor} shadow-2xl`}
      >
        {/* Close button */}
        <button
          onClick={onClose}
          className="absolute top-3 right-3 w-8 h-8 rounded-full bg-black/30 hover:bg-black/50 text-white text-lg flex items-center justify-center transition-colors"
          aria-label="Close"
        >
          ×
        </button>

        {/* Slide content */}
        <div className="flex flex-col items-center text-center gap-4 py-4">
          <div className="text-7xl">{slide.emoji}</div>
          <h2 className="text-2xl font-black text-white">{slide.title}</h2>
          <p className="text-white/90 text-base leading-relaxed px-2">
            {slide.body}
          </p>
        </div>

        {/* Progress dots */}
        <div className="flex justify-center gap-2 mt-6 mb-4">
          {SLIDES.map((_, i) => (
            <button
              key={i}
              onClick={() => setIndex(i)}
              className={`h-2 rounded-full transition-all ${
                i === index ? "w-8 bg-white" : "w-2 bg-white/40"
              }`}
              aria-label={`Go to slide ${i + 1}`}
            />
          ))}
        </div>

        {/* Navigation buttons */}
        <div className="flex gap-3">
          <button
            onClick={prev}
            disabled={isFirst}
            className="flex-1 px-4 py-3 rounded-xl bg-white/20 hover:bg-white/30 disabled:opacity-30 disabled:cursor-not-allowed text-white font-bold transition-colors"
          >
            ← Back
          </button>
          <button
            onClick={next}
            className="flex-1 px-4 py-3 rounded-xl bg-white hover:bg-white/90 text-gray-900 font-black transition-colors"
          >
            {isLast ? "Let's Play!" : "Next →"}
          </button>
        </div>
      </div>
    </div>
  );
}
