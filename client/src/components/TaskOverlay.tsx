"use client";

import { useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import TapTargets from "./tasks/TapTargets";
import ConnectWires from "./tasks/ConnectWires";
import MatchColors from "./tasks/MatchColors";
import SimonSays from "./tasks/SimonSays";
import Confetti from "./Confetti";
import type { MsgType } from "@/lib/protocol";

interface Props {
  send: (type: MsgType, payload?: unknown) => void;
}

export default function TaskOverlay({ send }: Props) {
  const { activeTask, setActiveTask } = useGameStore();
  const [celebrating, setCelebrating] = useState(false);

  if (!activeTask) return null;

  const handleComplete = () => {
    if (celebrating) return;
    send("task_complete", { stationId: activeTask.stationId });
    setCelebrating(true);
    // Briefly show confetti, then close the overlay
    setTimeout(() => {
      setCelebrating(false);
      setActiveTask(null);
    }, 1200);
  };

  const handleClose = () => {
    setActiveTask(null);
  };

  const renderTask = () => {
    const params = activeTask.params as Record<string, unknown>;

    switch (activeTask.type) {
      case "tap_targets":
        return (
          <TapTargets
            params={params as { targets: { x: number; y: number }[] }}
            onComplete={handleComplete}
          />
        );
      case "connect_wires":
        return (
          <ConnectWires
            params={
              params as { leftColors: string[]; rightColors: string[] }
            }
            onComplete={handleComplete}
          />
        );
      case "match_colors":
        return (
          <MatchColors
            params={params as { cards: string[] }}
            onComplete={handleComplete}
          />
        );
      case "simon_says":
        return (
          <SimonSays
            params={params as { sequence: string[] }}
            onComplete={handleComplete}
          />
        );
      default:
        return <p className="text-white">Unknown task type</p>;
    }
  };

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="flex flex-col items-center gap-4 relative">
        {celebrating ? (
          <div className="flex flex-col items-center gap-3 px-12 py-10 rounded-3xl bg-gradient-to-br from-green-600 to-emerald-700 shadow-2xl">
            <div className="text-6xl">🎉</div>
            <p className="text-white font-black text-2xl">Task Complete!</p>
          </div>
        ) : (
          <>
            {renderTask()}
            <button
              onClick={handleClose}
              className="px-6 py-2 text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors text-sm"
            >
              Cancel
            </button>
          </>
        )}
        {celebrating && <Confetti />}
      </div>
    </div>
  );
}
