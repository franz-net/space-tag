"use client";

import { useState, useEffect, useCallback } from "react";

const BUTTON_COLORS: Record<string, { bg: string; active: string }> = {
  red: { bg: "#991B1B", active: "#EF4444" },
  blue: { bg: "#1E3A5F", active: "#3B82F6" },
  green: { bg: "#14532D", active: "#22C55E" },
  yellow: { bg: "#713F12", active: "#EAB308" },
};

const BUTTONS = ["red", "blue", "green", "yellow"];

interface Props {
  params: { sequence: string[] };
  onComplete: () => void;
}

export default function SimonSays({ params, onComplete }: Props) {
  const [phase, setPhase] = useState<"showing" | "input">("showing");
  const [showIndex, setShowIndex] = useState(-1);
  const [inputIndex, setInputIndex] = useState(0);
  const [lit, setLit] = useState<string | null>(null);
  const [wrong, setWrong] = useState(false);

  // Show the sequence
  useEffect(() => {
    if (phase !== "showing") return;

    let step = 0;
    const interval = setInterval(() => {
      if (step < params.sequence.length) {
        setShowIndex(step);
        setLit(params.sequence[step]);
        setTimeout(() => setLit(null), 400);
        step++;
      } else {
        clearInterval(interval);
        setPhase("input");
        setShowIndex(-1);
      }
    }, 700);

    return () => clearInterval(interval);
  }, [phase, params.sequence]);

  const handlePress = useCallback(
    (color: string) => {
      if (phase !== "input") return;

      setLit(color);
      setTimeout(() => setLit(null), 200);

      if (color === params.sequence[inputIndex]) {
        if (inputIndex + 1 >= params.sequence.length) {
          setTimeout(onComplete, 400);
        } else {
          setInputIndex(inputIndex + 1);
        }
      } else {
        // Wrong — restart
        setWrong(true);
        setTimeout(() => {
          setWrong(false);
          setInputIndex(0);
          setPhase("showing");
        }, 800);
      }
    },
    [phase, inputIndex, params.sequence, onComplete]
  );

  return (
    <div className="w-[280px] bg-gray-800 rounded-2xl p-6">
      <p className="text-center text-gray-400 text-sm mb-4">
        {phase === "showing"
          ? "Watch the pattern..."
          : wrong
          ? "Oops! Try again..."
          : `Repeat it! (${inputIndex + 1}/${params.sequence.length})`}
      </p>
      <div className="grid grid-cols-2 gap-3">
        {BUTTONS.map((color) => (
          <button
            key={color}
            onClick={() => handlePress(color)}
            disabled={phase === "showing"}
            className="w-28 h-28 rounded-xl transition-all duration-150 active:scale-95"
            style={{
              backgroundColor:
                lit === color
                  ? BUTTON_COLORS[color].active
                  : BUTTON_COLORS[color].bg,
              boxShadow:
                lit === color ? `0 0 20px ${BUTTON_COLORS[color].active}` : "none",
            }}
          />
        ))}
      </div>
    </div>
  );
}
