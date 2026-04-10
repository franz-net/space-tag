"use client";

import { useEffect, useState } from "react";

/**
 * Returns true when the primary input is touch (phones, tablets incl. iPad).
 * We check `pointer: coarse` rather than screen width because iPads in
 * landscape are wider than Tailwind's `md` breakpoint, so width-based
 * detection wrongly classifies them as desktop.
 *
 * Starts as `false` on the server/first render to avoid hydration mismatches,
 * then flips on mount once we can actually query the media.
 */
export function useIsTouch(): boolean {
  const [isTouch, setIsTouch] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia("(pointer: coarse)");
    setIsTouch(mq.matches);
    const handler = (e: MediaQueryListEvent) => setIsTouch(e.matches);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, []);

  return isTouch;
}
