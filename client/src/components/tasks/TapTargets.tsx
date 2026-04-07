"use client";

import { useState, useEffect, useCallback } from "react";

interface Props {
  params: { targets: { x: number; y: number }[] };
  onComplete: () => void;
}

export default function TapTargets({ params, onComplete }: Props) {
  const [currentIndex, setCurrentIndex] = useState(0);
  const [tapped, setTapped] = useState<boolean[]>(
    new Array(params.targets.length).fill(false)
  );
  const [showBurst, setShowBurst] = useState<number | null>(null);

  const handleTap = useCallback(
    (index: number) => {
      if (index !== currentIndex || tapped[index]) return;

      const newTapped = [...tapped];
      newTapped[index] = true;
      setTapped(newTapped);
      setShowBurst(index);

      setTimeout(() => setShowBurst(null), 300);

      if (currentIndex + 1 >= params.targets.length) {
        setTimeout(onComplete, 400);
      } else {
        setCurrentIndex(currentIndex + 1);
      }
    },
    [currentIndex, tapped, params.targets.length, onComplete]
  );

  return (
    <div className="relative w-[300px] h-[300px] bg-gray-800 rounded-2xl overflow-hidden">
      <p className="absolute top-2 left-0 right-0 text-center text-gray-400 text-sm">
        Tap the stars! ({currentIndex}/{params.targets.length})
      </p>
      {params.targets.map((t, i) => (
        <button
          key={i}
          onClick={() => handleTap(i)}
          className={`absolute w-12 h-12 flex items-center justify-center text-2xl transition-all duration-200 ${
            tapped[i]
              ? "scale-0 opacity-0"
              : i === currentIndex
              ? "scale-100 animate-pulse"
              : "scale-0 opacity-0"
          }`}
          style={{ left: t.x - 24, top: t.y - 24 }}
        >
          {showBurst === i ? "✨" : "⭐"}
        </button>
      ))}
    </div>
  );
}
