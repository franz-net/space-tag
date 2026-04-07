"use client";

import { useState, useCallback } from "react";

const CARD_COLORS: Record<string, string> = {
  red: "#EF4444",
  blue: "#3B82F6",
  green: "#22C55E",
};

interface Props {
  params: { cards: string[] };
  onComplete: () => void;
}

export default function MatchColors({ params, onComplete }: Props) {
  const [flipped, setFlipped] = useState<boolean[]>(
    new Array(params.cards.length).fill(false)
  );
  const [matched, setMatched] = useState<boolean[]>(
    new Array(params.cards.length).fill(false)
  );
  const [selected, setSelected] = useState<number[]>([]);
  const [checking, setChecking] = useState(false);

  const handleFlip = useCallback(
    (index: number) => {
      if (checking || flipped[index] || matched[index]) return;

      const newFlipped = [...flipped];
      newFlipped[index] = true;
      setFlipped(newFlipped);

      const newSelected = [...selected, index];
      setSelected(newSelected);

      if (newSelected.length === 2) {
        setChecking(true);
        const [a, b] = newSelected;

        if (params.cards[a] === params.cards[b]) {
          // Match!
          const newMatched = [...matched];
          newMatched[a] = true;
          newMatched[b] = true;
          setMatched(newMatched);
          setSelected([]);
          setChecking(false);

          if (newMatched.filter(Boolean).length === params.cards.length) {
            setTimeout(onComplete, 400);
          }
        } else {
          // No match — flip back
          setTimeout(() => {
            const resetFlipped = [...newFlipped];
            resetFlipped[a] = false;
            resetFlipped[b] = false;
            setFlipped(resetFlipped);
            setSelected([]);
            setChecking(false);
          }, 800);
        }
      }
    },
    [checking, flipped, matched, selected, params.cards, onComplete]
  );

  return (
    <div className="w-[280px] bg-gray-800 rounded-2xl p-6">
      <p className="text-center text-gray-400 text-sm mb-4">
        Find the matching pairs!
      </p>
      <div className="grid grid-cols-3 gap-3">
        {params.cards.map((color, i) => (
          <button
            key={i}
            onClick={() => handleFlip(i)}
            className={`w-20 h-20 rounded-xl transition-all duration-300 ${
              matched[i]
                ? "scale-90 opacity-60"
                : "hover:scale-105 active:scale-95"
            }`}
            style={{
              backgroundColor:
                flipped[i] || matched[i] ? CARD_COLORS[color] : "#4A5568",
            }}
          >
            {!flipped[i] && !matched[i] && (
              <span className="text-2xl">❓</span>
            )}
            {matched[i] && <span className="text-2xl">✅</span>}
          </button>
        ))}
      </div>
    </div>
  );
}
