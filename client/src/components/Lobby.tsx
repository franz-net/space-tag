"use client";

import { useGameStore } from "@/stores/gameStore";
import { sounds } from "@/lib/sounds";
import { COLOR_HEX, type MsgType } from "@/lib/protocol";

interface LobbyProps {
  send: (type: MsgType, payload?: unknown) => void;
  connected: boolean;
}

export default function Lobby({ send, connected }: LobbyProps) {
  const { roomCode, players, hostId, myId, leaveRoom } = useGameStore();
  const isHost = myId === hostId;
  const canStart = players.length >= 2;
  const aiCount = players.filter((p) => p.isAI).length;
  const canAddAI = players.length < 6;

  return (
    <div className="flex flex-col items-center gap-8 p-8">
      {/* Room Code */}
      <div className="text-center">
        <p className="text-lg text-gray-400 mb-1">Room Code</p>
        <div className="text-6xl font-bold tracking-[0.3em] text-white bg-gray-800 px-8 py-4 rounded-2xl select-all">
          {roomCode}
        </div>
        <p className="text-sm text-gray-500 mt-2">
          Share this code with friends to join!
        </p>
      </div>

      {/* Players */}
      <div className="w-full max-w-md">
        <h2 className="text-xl font-semibold text-white mb-4 text-center">
          Players ({players.length}/6)
        </h2>
        <div className="grid grid-cols-2 gap-3">
          {players.map((player) => (
            <div
              key={player.id}
              className="flex items-center gap-3 bg-gray-800 rounded-xl px-4 py-3"
            >
              <div
                className="w-10 h-10 rounded-full flex items-center justify-center text-white font-bold text-lg shrink-0"
                style={{ backgroundColor: COLOR_HEX[player.color] }}
              >
                {player.isAI ? "🤖" : player.name[0]?.toUpperCase()}
              </div>
              <div className="min-w-0">
                <p className="text-white font-medium truncate">
                  {player.name}
                  {player.id === myId && (
                    <span className="text-gray-400 text-sm"> (you)</span>
                  )}
                </p>
                <p className="text-gray-400 text-xs">
                  {player.isHost ? "⭐ Host" : player.isAI ? "🤖 Bot" : "Player"}
                </p>
              </div>
            </div>
          ))}

          {/* Empty slots */}
          {Array.from({ length: 6 - players.length }).map((_, i) => (
            <div
              key={`empty-${i}`}
              className="flex items-center gap-3 bg-gray-800/30 rounded-xl px-4 py-3 border-2 border-dashed border-gray-700"
            >
              <div className="w-10 h-10 rounded-full bg-gray-700/50 shrink-0" />
              <p className="text-gray-600 text-sm">Waiting...</p>
            </div>
          ))}
        </div>
      </div>

      {/* Host Controls */}
      {isHost && (
        <div className="flex flex-col gap-3 w-full max-w-md">
          <div className="flex gap-3">
            <button
              onClick={() => send("add_ai")}
              disabled={!canAddAI}
              className="flex-1 px-4 py-3 bg-gray-700 hover:bg-gray-600 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-xl font-medium transition-colors"
            >
              + Add Bot
            </button>
            <button
              onClick={() => send("remove_ai")}
              disabled={aiCount === 0}
              className="flex-1 px-4 py-3 bg-gray-700 hover:bg-gray-600 disabled:opacity-40 disabled:cursor-not-allowed text-white rounded-xl font-medium transition-colors"
            >
              - Remove Bot
            </button>
          </div>
          <button
            onClick={() => {
              sounds.unlock();
              send("start_game");
            }}
            disabled={!canStart}
            className="w-full px-6 py-4 bg-green-600 hover:bg-green-500 disabled:bg-gray-700 disabled:cursor-not-allowed text-white rounded-xl font-bold text-xl transition-colors"
          >
            {canStart ? "Start Game!" : "Need at least 2 players"}
          </button>
        </div>
      )}

      {!isHost && (
        <p className="text-gray-400 text-lg">
          Waiting for the host to start the game...
        </p>
      )}

      {!connected && (
        <div className="px-4 py-2 bg-yellow-500/20 border border-yellow-500/50 rounded-xl text-yellow-300 text-sm">
          Reconnecting...
        </div>
      )}

      {/* Leave button */}
      <button
        onClick={() => {
          send("leave_room");
          leaveRoom();
        }}
        className="px-6 py-2 text-gray-400 hover:text-white hover:bg-gray-800 rounded-lg transition-colors"
      >
        Leave Room
      </button>
    </div>
  );
}
