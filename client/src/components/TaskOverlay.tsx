"use client";

import { useGameStore } from "@/stores/gameStore";
import TapTargets from "./tasks/TapTargets";
import ConnectWires from "./tasks/ConnectWires";
import MatchColors from "./tasks/MatchColors";
import SimonSays from "./tasks/SimonSays";
import type { MsgType } from "@/lib/protocol";

interface Props {
  send: (type: MsgType, payload?: unknown) => void;
}

export default function TaskOverlay({ send }: Props) {
  const { activeTask, setActiveTask } = useGameStore();

  if (!activeTask) return null;

  const handleComplete = () => {
    send("task_complete", { stationId: activeTask.stationId });
    setActiveTask(null);
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
      <div className="flex flex-col items-center gap-4">
        {renderTask()}
        <button
          onClick={handleClose}
          className="px-6 py-2 text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors text-sm"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
