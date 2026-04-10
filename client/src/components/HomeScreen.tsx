"use client";

import { useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { sounds } from "@/lib/sounds";
import HowToPlay from "./HowToPlay";
import MuteButton from "./MuteButton";
import type { MsgType } from "@/lib/protocol";

interface HomeScreenProps {
  send: (type: MsgType, payload?: unknown) => void;
  connected: boolean;
}

export default function HomeScreen({ send, connected }: HomeScreenProps) {
  const [joinCode, setJoinCode] = useState("");
  const [showHowTo, setShowHowTo] = useState(false);
  const { playerName, setPlayerName, setError, error, clearError } =
    useGameStore();

  const handleCreate = () => {
    // Unlock audio synchronously inside this gesture callback — iOS Safari
    // is strict about which call stack creates the AudioContext.
    sounds.unlock();
    if (!playerName.trim()) {
      setError("Please enter your name!");
      return;
    }
    clearError();
    send("create_room", { playerName: playerName.trim() });
  };

  const handleJoin = () => {
    sounds.unlock();
    if (!playerName.trim()) {
      setError("Please enter your name!");
      return;
    }
    if (!joinCode.trim()) {
      setError("Please enter a room code!");
      return;
    }
    clearError();
    send("join_room", {
      roomCode: joinCode.trim().toUpperCase(),
      playerName: playerName.trim(),
    });
  };

  return (
    <div className="w-full max-w-sm flex flex-col items-center gap-8">
      {/* Title */}
      <div className="text-center">
        <h1 className="text-6xl font-black text-white mb-2">
          Space<span className="text-blue-400">Tag</span>
        </h1>
        <p className="text-gray-400 text-lg">
          A friendly game of hide &amp; seek in space!
        </p>
      </div>

      {/* Connection status */}
      <div className="flex items-center gap-2">
        <div
          className={`w-2.5 h-2.5 rounded-full ${
            connected ? "bg-green-500" : "bg-red-500 animate-pulse"
          }`}
        />
        <span className="text-sm text-gray-400">
          {connected ? "Connected" : "Connecting..."}
        </span>
      </div>

      {/* Name input */}
      <div className="w-full">
        <label className="block text-gray-300 text-sm font-medium mb-2">
          Your Name
        </label>
        <input
          type="text"
          value={playerName}
          onChange={(e) => setPlayerName(e.target.value)}
          placeholder="Enter your name..."
          maxLength={12}
          className="w-full px-4 py-3 bg-gray-800 text-white text-lg rounded-xl border-2 border-gray-700 focus:border-blue-500 focus:outline-none placeholder:text-gray-500"
          onKeyDown={(e) => e.key === "Enter" && handleCreate()}
        />
      </div>

      {/* Create Room */}
      <button
        onClick={handleCreate}
        disabled={!connected}
        className="w-full px-6 py-4 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-700 disabled:cursor-not-allowed text-white font-bold text-xl rounded-xl transition-colors"
      >
        Create Room
      </button>

      {/* Divider */}
      <div className="flex items-center gap-4 w-full">
        <div className="flex-1 h-px bg-gray-700" />
        <span className="text-gray-500 text-sm">or join a friend</span>
        <div className="flex-1 h-px bg-gray-700" />
      </div>

      {/* Join Room */}
      <div className="w-full flex gap-3">
        <input
          type="text"
          value={joinCode}
          onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
          placeholder="CODE"
          maxLength={4}
          className="flex-1 px-4 py-3 bg-gray-800 text-white text-lg text-center tracking-[0.2em] rounded-xl border-2 border-gray-700 focus:border-blue-500 focus:outline-none placeholder:text-gray-500 uppercase"
          onKeyDown={(e) => e.key === "Enter" && handleJoin()}
        />
        <button
          onClick={handleJoin}
          disabled={!connected}
          className="px-6 py-3 bg-green-600 hover:bg-green-500 disabled:bg-gray-700 disabled:cursor-not-allowed text-white font-bold rounded-xl transition-colors"
        >
          Join
        </button>
      </div>

      {/* Error */}
      {error && (
        <div className="w-full px-4 py-3 bg-red-500/20 border border-red-500/50 rounded-xl text-red-300 text-center">
          {error}
        </div>
      )}

      {/* How to play link */}
      <button
        onClick={() => setShowHowTo(true)}
        className="text-gray-400 hover:text-white text-sm underline transition-colors"
      >
        How to play
      </button>

      {showHowTo && <HowToPlay onClose={() => setShowHowTo(false)} />}

      {/* Mute toggle — fixed to top-right */}
      <div className="fixed top-4 right-4 z-20">
        <MuteButton />
      </div>
    </div>
  );
}
