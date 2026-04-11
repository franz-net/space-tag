"use client";

import { useState, useEffect } from "react";
import { sounds } from "@/lib/sounds";
import { useIsTouch } from "@/hooks/useIsTouch";
import { useTouchMode, setTouchMode } from "@/hooks/useTouchMode";
import HowToPlay from "./HowToPlay";

export default function SettingsMenu() {
  const [open, setOpen] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const [muted, setMuted] = useState(false);
  const [musicMuted, setMusicMuted] = useState(false);
  const isTouch = useIsTouch();
  const touchMode = useTouchMode();

  useEffect(() => {
    setMuted(sounds.isMuted());
    setMusicMuted(sounds.isMusicMuted());
  }, []);

  const toggleMute = () => {
    const newMuted = sounds.toggleMute();
    setMuted(newMuted);
    if (!newMuted) sounds.click();
  };

  const toggleMusic = () => {
    const newMuted = sounds.toggleMusic();
    setMusicMuted(newMuted);
  };

  return (
    <>
      <button
        onClick={() => {
          sounds.click();
          setOpen(!open);
        }}
        aria-label="Settings"
        title="Settings"
        className="w-9 h-9 rounded-full bg-black/40 hover:bg-black/60 backdrop-blur flex items-center justify-center text-lg transition-colors"
      >
        ⚙️
      </button>

      {open && (
        <>
          {/* Click-outside catcher */}
          <div
            className="fixed inset-0 z-20"
            onClick={() => setOpen(false)}
          />
          <div className="absolute top-full left-0 mt-2 z-30 w-48 rounded-xl bg-gray-900/95 backdrop-blur shadow-2xl border border-gray-700 p-2 flex flex-col gap-1">
            <button
              onClick={toggleMute}
              className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800 text-white text-sm text-left"
            >
              <span className="text-lg">{muted ? "🔇" : "🔊"}</span>
              <span className="flex-1">Sound</span>
              <span className="text-xs text-gray-400">
                {muted ? "Off" : "On"}
              </span>
            </button>
            <button
              onClick={toggleMusic}
              className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800 text-white text-sm text-left"
            >
              <span className="text-lg">{musicMuted ? "🎵" : "🎶"}</span>
              <span className="flex-1">Music</span>
              <span className="text-xs text-gray-400">
                {musicMuted ? "Off" : "On"}
              </span>
            </button>
            {/* Touch controls toggle — only on touch devices */}
            {isTouch && (
              <button
                onClick={() => {
                  sounds.click();
                  setTouchMode(touchMode === "joystick" ? "follow" : "joystick");
                }}
                className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800 text-white text-sm text-left"
              >
                <span className="text-lg">{touchMode === "joystick" ? "🕹️" : "👆"}</span>
                <span className="flex-1">Controls</span>
                <span className="text-xs text-gray-400">
                  {touchMode === "joystick" ? "Joystick" : "Follow"}
                </span>
              </button>
            )}
            <button
              onClick={() => {
                sounds.click();
                setShowHelp(true);
                setOpen(false);
              }}
              className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800 text-white text-sm text-left"
            >
              <span className="text-lg">❓</span>
              <span className="flex-1">How to Play</span>
            </button>
          </div>
        </>
      )}

      {showHelp && <HowToPlay onClose={() => setShowHelp(false)} />}
    </>
  );
}
