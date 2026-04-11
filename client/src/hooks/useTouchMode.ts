"use client";

import { useSyncExternalStore } from "react";

export type TouchMode = "joystick" | "follow";

const STORAGE_KEY = "spacetag.touchMode";

function getSnapshot(): TouchMode {
  if (typeof window === "undefined") return "joystick";
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === "follow" ? "follow" : "joystick";
}

function getServerSnapshot(): TouchMode {
  return "joystick";
}

const listeners = new Set<() => void>();
function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => listeners.delete(cb);
}

export function setTouchMode(mode: TouchMode) {
  localStorage.setItem(STORAGE_KEY, mode);
  listeners.forEach((cb) => cb());
}

export function useTouchMode(): TouchMode {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
