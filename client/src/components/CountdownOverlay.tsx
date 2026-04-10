"use client";

import { useEffect, useState } from "react";

interface Props {
  onDone: () => void;
}

// 3 → 2 → 1 → GO! shown over 4 seconds. Each step is 1 second.
const STEPS = ["3", "2", "1", "GO!"];

export default function CountdownOverlay({ onDone }: Props) {
  const [step, setStep] = useState(0);

  useEffect(() => {
    if (step >= STEPS.length) {
      onDone();
      return;
    }
    const id = setTimeout(() => setStep(step + 1), 900);
    return () => clearTimeout(id);
  }, [step, onDone]);

  if (step >= STEPS.length) return null;
  const label = STEPS[step];
  const isGo = label === "GO!";

  return (
    <div className="absolute inset-0 z-40 flex items-center justify-center pointer-events-none">
      <div
        key={step}
        className={`text-white font-black drop-shadow-[0_0_24px_rgba(0,0,0,0.8)] animate-countdown-pop ${
          isGo ? "text-9xl text-green-300" : "text-[10rem]"
        }`}
      >
        {label}
      </div>
    </div>
  );
}
