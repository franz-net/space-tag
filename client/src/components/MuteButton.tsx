"use client";

import { useState, useEffect } from "react";
import { sounds } from "@/lib/sounds";

interface Props {
  className?: string;
}

export default function MuteButton({ className = "" }: Props) {
  const [muted, setMuted] = useState(false);

  // Initialize from sound manager (which reads localStorage)
  useEffect(() => {
    setMuted(sounds.isMuted());
  }, []);

  const handleClick = () => {
    const newMuted = sounds.toggleMute();
    setMuted(newMuted);
    if (!newMuted) {
      // Confirm sound is on with a quick click
      sounds.click();
    }
  };

  return (
    <button
      onClick={handleClick}
      aria-label={muted ? "Unmute sounds" : "Mute sounds"}
      title={muted ? "Sounds off" : "Sounds on"}
      className={`w-9 h-9 rounded-full bg-black/40 hover:bg-black/60 backdrop-blur flex items-center justify-center text-lg transition-colors ${className}`}
    >
      {muted ? "🔇" : "🔊"}
    </button>
  );
}
