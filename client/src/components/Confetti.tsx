"use client";

import { useMemo } from "react";

const COLORS = [
  "#EF4444",
  "#3B82F6",
  "#22C55E",
  "#EAB308",
  "#A855F7",
  "#F97316",
  "#FFFFFF",
];

interface Props {
  count?: number;
}

// Pure-CSS confetti — generates `count` little squares with randomized
// horizontal drift, color, size, and start delay. Each piece falls via the
// `confetti-fall` keyframe defined in globals.css.
export default function Confetti({ count = 40 }: Props) {
  const pieces = useMemo(() => {
    return Array.from({ length: count }).map((_, i) => {
      const size = 6 + Math.random() * 8;
      const left = Math.random() * 100;
      const dx = (Math.random() - 0.5) * 200;
      const delay = Math.random() * 0.3;
      const color = COLORS[i % COLORS.length];
      return { size, left, dx, delay, color };
    });
  }, [count]);

  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden">
      {pieces.map((p, i) => (
        <span
          key={i}
          className="absolute top-0 animate-confetti rounded-sm"
          style={
            {
              left: `${p.left}%`,
              width: `${p.size}px`,
              height: `${p.size}px`,
              backgroundColor: p.color,
              animationDelay: `${p.delay}s`,
              // CSS variable consumed by the keyframe
              "--dx": `${p.dx}px`,
            } as React.CSSProperties
          }
        />
      ))}
    </div>
  );
}
