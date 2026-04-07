"use client";

import { useState, useRef, useCallback, useEffect } from "react";

const WIRE_COLORS: Record<string, string> = {
  red: "#EF4444",
  blue: "#3B82F6",
  green: "#22C55E",
  yellow: "#EAB308",
};

interface Props {
  params: { leftColors: string[]; rightColors: string[] };
  onComplete: () => void;
}

interface Point {
  x: number;
  y: number;
}

export default function ConnectWires({ params, onComplete }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const leftRefs = useRef<(HTMLDivElement | null)[]>([]);
  const rightRefs = useRef<(HTMLDivElement | null)[]>([]);

  // Each entry maps left index → right index when connected
  const [connections, setConnections] = useState<Record<number, number>>({});
  const [dragging, setDragging] = useState<{
    leftIndex: number;
    color: string;
    start: Point;
    current: Point;
  } | null>(null);

  // Get center of an element relative to the container
  const getCenter = useCallback((el: HTMLElement | null): Point | null => {
    if (!el || !containerRef.current) return null;
    const elRect = el.getBoundingClientRect();
    const containerRect = containerRef.current.getBoundingClientRect();
    return {
      x: elRect.left + elRect.width / 2 - containerRect.left,
      y: elRect.top + elRect.height / 2 - containerRect.top,
    };
  }, []);

  const handlePointerDown = (e: React.PointerEvent, leftIndex: number) => {
    if (connections[leftIndex] !== undefined) return;
    e.preventDefault();
    const start = getCenter(leftRefs.current[leftIndex]);
    if (!start || !containerRef.current) return;

    const containerRect = containerRef.current.getBoundingClientRect();
    setDragging({
      leftIndex,
      color: params.leftColors[leftIndex],
      start,
      current: {
        x: e.clientX - containerRect.left,
        y: e.clientY - containerRect.top,
      },
    });
  };

  const handlePointerMove = useCallback(
    (e: PointerEvent) => {
      if (!dragging || !containerRef.current) return;
      const containerRect = containerRef.current.getBoundingClientRect();
      setDragging({
        ...dragging,
        current: {
          x: e.clientX - containerRect.left,
          y: e.clientY - containerRect.top,
        },
      });
    },
    [dragging]
  );

  const handlePointerUp = useCallback(
    (e: PointerEvent) => {
      if (!dragging) return;

      // Find which right socket the pointer is over
      let droppedOn: number | null = null;
      for (let i = 0; i < rightRefs.current.length; i++) {
        const el = rightRefs.current[i];
        if (!el) continue;
        const r = el.getBoundingClientRect();
        if (
          e.clientX >= r.left &&
          e.clientX <= r.right &&
          e.clientY >= r.top &&
          e.clientY <= r.bottom
        ) {
          droppedOn = i;
          break;
        }
      }

      // Only connect if matching color and not already taken
      if (
        droppedOn !== null &&
        params.rightColors[droppedOn] === dragging.color &&
        !Object.values(connections).includes(droppedOn)
      ) {
        const newConnections = {
          ...connections,
          [dragging.leftIndex]: droppedOn,
        };
        setConnections(newConnections);

        if (Object.keys(newConnections).length === params.leftColors.length) {
          setTimeout(onComplete, 400);
        }
      }

      setDragging(null);
    },
    [dragging, connections, params, onComplete]
  );

  // Attach global pointer move/up listeners while dragging
  useEffect(() => {
    if (!dragging) return;
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
  }, [dragging, handlePointerMove, handlePointerUp]);

  // Recompute completed connection lines on each render so they follow the actual element positions
  const completedLines: { x1: number; y1: number; x2: number; y2: number; color: string }[] = [];
  for (const [leftIdxStr, rightIdx] of Object.entries(connections)) {
    const leftIdx = Number(leftIdxStr);
    const a = getCenter(leftRefs.current[leftIdx]);
    const b = getCenter(rightRefs.current[rightIdx]);
    if (a && b) {
      completedLines.push({
        x1: a.x,
        y1: a.y,
        x2: b.x,
        y2: b.y,
        color: WIRE_COLORS[params.leftColors[leftIdx]],
      });
    }
  }

  return (
    <div
      ref={containerRef}
      className="relative w-[320px] bg-gray-800 rounded-2xl p-6 select-none touch-none"
    >
      <p className="text-center text-gray-400 text-sm mb-4">
        Drag wires to matching colors!
      </p>
      <div className="flex justify-between gap-12">
        {/* Left side */}
        <div className="flex flex-col gap-4">
          {params.leftColors.map((color, i) => (
            <div
              key={`l-${i}`}
              ref={(el) => {
                leftRefs.current[i] = el;
              }}
              onPointerDown={(e) => handlePointerDown(e, i)}
              className={`w-14 h-10 rounded-lg cursor-grab active:cursor-grabbing transition-all ${
                connections[i] !== undefined ? "opacity-50" : "hover:scale-105"
              }`}
              style={{ backgroundColor: WIRE_COLORS[color] }}
            />
          ))}
        </div>

        {/* Right side */}
        <div className="flex flex-col gap-4">
          {params.rightColors.map((color, i) => (
            <div
              key={`r-${i}`}
              ref={(el) => {
                rightRefs.current[i] = el;
              }}
              className={`w-14 h-10 rounded-lg transition-all ${
                Object.values(connections).includes(i) ? "opacity-50" : ""
              }`}
              style={{ backgroundColor: WIRE_COLORS[color] }}
            />
          ))}
        </div>
      </div>

      {/* SVG overlay for wire lines */}
      <svg className="absolute inset-0 w-full h-full pointer-events-none">
        {completedLines.map((line, i) => (
          <line
            key={`done-${i}`}
            x1={line.x1}
            y1={line.y1}
            x2={line.x2}
            y2={line.y2}
            stroke={line.color}
            strokeWidth={5}
            strokeLinecap="round"
          />
        ))}
        {dragging && (
          <line
            x1={dragging.start.x}
            y1={dragging.start.y}
            x2={dragging.current.x}
            y2={dragging.current.y}
            stroke={WIRE_COLORS[dragging.color]}
            strokeWidth={5}
            strokeLinecap="round"
            strokeDasharray="6 4"
          />
        )}
      </svg>
    </div>
  );
}
