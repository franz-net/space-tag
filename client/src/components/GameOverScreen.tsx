"use client";

import { useGameStore } from "@/stores/gameStore";
import type { MsgType } from "@/lib/protocol";

interface Props {
  send: (type: MsgType, payload?: unknown) => void;
}

export default function GameOverScreen({ send }: Props) {
  const { gameOver, returnToLobby, leaveRoom } = useGameStore();

  if (!gameOver) return null;

  const crewWon = gameOver.winner === "crew";

  const handlePlayAgain = () => {
    // Stay in the same room — just dismiss the overlay and go back to the lobby
    returnToLobby();
  };

  const handleLeave = () => {
    // Tell server we're leaving and reset client state to home
    send("leave_room");
    leaveRoom();
  };

  return (
    <div className="absolute inset-0 z-40 flex items-center justify-center bg-black/80 backdrop-blur-sm">
      <div className="flex flex-col items-center gap-6 bg-gray-900 rounded-2xl p-8 max-w-sm w-full mx-4">
        <h1 className="text-4xl font-black text-center">
          {crewWon ? (
            <span className="text-blue-400">Crew saves the day!</span>
          ) : (
            <span className="text-red-400">The Tagger wins!</span>
          )}
        </h1>

        <div className="w-full">
          <h2 className="text-gray-400 text-sm font-medium mb-3">
            Roles Revealed
          </h2>
          <div className="flex flex-col gap-2">
            {gameOver.roles.map((r) => (
              <div
                key={r.id}
                className="flex items-center justify-between px-4 py-2 rounded-lg bg-gray-800"
              >
                <span className="text-white font-medium">{r.name}</span>
                <span
                  className={`font-bold text-sm ${
                    r.role === "tagger" ? "text-red-400" : "text-blue-400"
                  }`}
                >
                  {r.role === "tagger" ? "Tagger" : "Crewmate"}
                </span>
              </div>
            ))}
          </div>
        </div>

        <div className="w-full flex flex-col gap-3">
          <button
            onClick={handlePlayAgain}
            className="w-full px-6 py-4 bg-blue-600 hover:bg-blue-500 text-white rounded-xl font-bold text-xl transition-colors"
          >
            Play Again
          </button>
          <button
            onClick={handleLeave}
            className="w-full px-6 py-2 bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-white rounded-xl text-sm transition-colors"
          >
            Leave Room
          </button>
        </div>
      </div>
    </div>
  );
}
