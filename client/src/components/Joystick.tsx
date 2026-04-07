"use client";

import { useRef, useState, useCallback, useEffect } from "react";

const OUTER_RADIUS = 60;
const INNER_RADIUS = 28;
const MAX_DIST = OUTER_RADIUS - INNER_RADIUS;

interface Props {
  onMove: (x: number, y: number) => void;
}

export default function Joystick({ onMove }: Props) {
  const outerRef = useRef<HTMLDivElement>(null);
  const [knob, setKnob] = useState({ x: 0, y: 0 });
  const [active, setActive] = useState(false);

  const updateFromPointer = useCallback(
    (clientX: number, clientY: number) => {
      const outer = outerRef.current;
      if (!outer) return;
      const rect = outer.getBoundingClientRect();
      const cx = rect.left + rect.width / 2;
      const cy = rect.top + rect.height / 2;
      let dx = clientX - cx;
      let dy = clientY - cy;
      const dist = Math.sqrt(dx * dx + dy * dy);

      if (dist > MAX_DIST) {
        dx = (dx / dist) * MAX_DIST;
        dy = (dy / dist) * MAX_DIST;
      }

      setKnob({ x: dx, y: dy });
      // Normalize to -1..1 for movement
      onMove(dx / MAX_DIST, dy / MAX_DIST);
    },
    [onMove]
  );

  const handlePointerDown = (e: React.PointerEvent) => {
    e.preventDefault();
    setActive(true);
    updateFromPointer(e.clientX, e.clientY);
  };

  const handlePointerMove = useCallback(
    (e: PointerEvent) => {
      if (!active) return;
      updateFromPointer(e.clientX, e.clientY);
    },
    [active, updateFromPointer]
  );

  const handlePointerUp = useCallback(() => {
    setActive(false);
    setKnob({ x: 0, y: 0 });
    onMove(0, 0);
  }, [onMove]);

  useEffect(() => {
    if (!active) return;
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    window.addEventListener("pointercancel", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.removeEventListener("pointercancel", handlePointerUp);
    };
  }, [active, handlePointerMove, handlePointerUp]);

  return (
    <div
      ref={outerRef}
      onPointerDown={handlePointerDown}
      className="md:hidden absolute bottom-6 left-6 z-20 rounded-full bg-white/10 backdrop-blur border-2 border-white/30 touch-none select-none"
      style={{ width: OUTER_RADIUS * 2, height: OUTER_RADIUS * 2 }}
    >
      <div
        className="absolute rounded-full bg-white/60 border-2 border-white/80 pointer-events-none"
        style={{
          width: INNER_RADIUS * 2,
          height: INNER_RADIUS * 2,
          left: OUTER_RADIUS - INNER_RADIUS + knob.x,
          top: OUTER_RADIUS - INNER_RADIUS + knob.y,
          transition: active ? "none" : "all 0.15s ease-out",
        }}
      />
    </div>
  );
}
