// Debug logging helper. Enable by setting window.spaceTagDebug = true in
// the browser console (or NEXT_PUBLIC_DEBUG=1 at build time).
//
// Logs are prefixed with [DBG] and a timestamp so they're easy to scan.

declare global {
  interface Window {
    spaceTagDebug?: boolean;
  }
}

export function isDebugEnabled(): boolean {
  if (typeof window === "undefined") return false;
  if (window.spaceTagDebug) return true;
  if (process.env.NEXT_PUBLIC_DEBUG === "1") return true;
  return false;
}

export function dbg(...args: unknown[]): void {
  if (!isDebugEnabled()) return;
  const t = new Date().toISOString().slice(11, 23); // HH:MM:SS.mmm
  // eslint-disable-next-line no-console
  console.log(`[DBG ${t}]`, ...args);
}

// Force-enable from URL: ?debug=1
if (typeof window !== "undefined") {
  try {
    const params = new URLSearchParams(window.location.search);
    if (params.get("debug") === "1") {
      window.spaceTagDebug = true;
    }
  } catch {
    // ignore
  }
}
