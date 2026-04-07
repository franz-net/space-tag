"use client";

import { useEffect, useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { sounds } from "@/lib/sounds";
import MuteButton from "./MuteButton";
import type { MsgType, TaskType } from "@/lib/protocol";

const TASK_ICONS: Record<TaskType, string> = {
  tap_targets: "⭐",
  connect_wires: "🔌",
  match_colors: "🎴",
  simon_says: "🎵",
};

const TASK_NAMES: Record<TaskType, string> = {
  tap_targets: "Tap stars",
  connect_wires: "Connect wires",
  match_colors: "Match colors",
  simon_says: "Simon says",
};

const ROOM_NAMES: Record<string, string> = {
  cafeteria: "Cafeteria",
  medbay: "Medbay",
  navigation: "Navigation",
  engine: "Engine",
  storage: "Storage",
  reactor: "Reactor",
};

interface Props {
  send: (type: MsgType, payload?: unknown) => void;
  nearTaskId: string | null;
  nearTagTargetId: string | null;
  nearBodyId: string | null;
  inCafeteria: boolean;
}

export default function HUD({
  send,
  nearTaskId,
  nearTagTargetId,
  nearBodyId,
  inCafeteria,
}: Props) {
  const {
    myRole,
    taskProgress,
    myTasks,
    tagCooldownEnd,
    usedEmergency,
    frozenIds,
    myId,
  } = useGameStore();

  const completedCount = myTasks.filter((t) => t.completed).length;
  const totalCount = myTasks.length;
  const isFrozen = myId ? frozenIds.has(myId) : false;

  // Live cooldown counter
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 100);
    return () => clearInterval(id);
  }, []);

  const cooldownRemaining = Math.max(0, (tagCooldownEnd - now) / 1000);

  // Soft ping when the tag cooldown just hit zero (tagger only)
  useEffect(() => {
    if (myRole !== "tagger" || tagCooldownEnd === 0) return;
    const remaining = tagCooldownEnd - Date.now();
    if (remaining <= 0) return;
    const id = setTimeout(() => sounds.tagReady(), remaining);
    return () => clearTimeout(id);
  }, [tagCooldownEnd, myRole]);
  const canTag = myRole === "tagger" && cooldownRemaining === 0 && nearTagTargetId !== null;
  const canEmergency = inCafeteria && !usedEmergency && !isFrozen;

  const handleUse = () => {
    if (!nearTaskId) return;
    sounds.click();
    send("task_start", { stationId: nearTaskId });
  };

  const handleTag = () => {
    if (!canTag || !nearTagTargetId) return;
    sounds.tag();
    send("tag_player", { targetId: nearTagTargetId });
  };

  const handleReport = () => {
    if (!nearBodyId) return;
    sounds.reportBody();
    send("report_body");
  };

  const handleEmergency = () => {
    if (!canEmergency) return;
    sounds.reportBody();
    send("emergency");
  };

  return (
    <>
      {/* Task progress bar — top center */}
      <div className="absolute top-4 left-1/2 -translate-x-1/2 z-10 flex items-center gap-3 px-4 py-2 rounded-xl bg-black/60 backdrop-blur">
        <span className="text-gray-300 text-sm font-medium">Tasks</span>
        <div className="w-48 h-3 bg-gray-700 rounded-full overflow-hidden">
          <div
            className="h-full bg-green-500 rounded-full transition-all duration-500"
            style={{ width: `${taskProgress * 100}%` }}
          />
        </div>
        <span className="text-gray-400 text-xs">
          {Math.round(taskProgress * 100)}%
        </span>
      </div>

      {/* Your tasks list — right side */}
      {myRole === "crewmate" && (
        <div className="absolute top-4 right-4 z-10 w-56 px-4 py-3 rounded-xl bg-black/60 backdrop-blur">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-gray-300 text-sm font-semibold">Your Tasks</h3>
            <span className="text-gray-500 text-xs">
              {completedCount}/{totalCount}
            </span>
          </div>
          <ul className="flex flex-col gap-1.5">
            {myTasks.map((task) => (
              <li
                key={task.stationId}
                className={`flex items-center gap-2 text-sm ${
                  task.completed ? "text-gray-500 line-through" : "text-white"
                }`}
              >
                <span className="text-base">
                  {task.completed ? "✅" : TASK_ICONS[task.type]}
                </span>
                <span className="flex-1 truncate">{TASK_NAMES[task.type]}</span>
                <span className="text-xs text-gray-400">
                  {ROOM_NAMES[task.roomId] || task.roomId}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Tagger fake task list */}
      {myRole === "tagger" && (
        <div className="absolute top-4 right-4 z-10 w-56 px-4 py-3 rounded-xl bg-black/60 backdrop-blur">
          <h3 className="text-red-400 text-sm font-semibold mb-2">
            You&apos;re the Tagger!
          </h3>
          <p className="text-gray-300 text-xs">
            Pretend to do tasks. Freeze crewmates when no one is watching.
          </p>
        </div>
      )}

      {/* Right side action buttons stack */}
      <div className="absolute bottom-6 right-6 z-10 flex flex-col items-end gap-3">
        {/* TAG button (tagger only) */}
        {myRole === "tagger" && !isFrozen && (
          <button
            onClick={handleTag}
            disabled={!canTag}
            className={`relative w-20 h-20 rounded-full font-bold text-lg transition-all ${
              canTag
                ? "bg-red-500 hover:bg-red-400 text-white animate-pulse shadow-lg shadow-red-500/50"
                : "bg-gray-700 text-gray-500 cursor-not-allowed"
            }`}
          >
            TAG
            {cooldownRemaining > 0 && (
              <span className="absolute inset-0 flex items-center justify-center text-2xl font-black text-white">
                {Math.ceil(cooldownRemaining)}
              </span>
            )}
          </button>
        )}

        {/* REPORT button (anyone alive) */}
        {!isFrozen && (
          <button
            onClick={handleReport}
            disabled={!nearBodyId}
            className={`w-20 h-20 rounded-full font-bold text-base transition-all ${
              nearBodyId
                ? "bg-blue-400 hover:bg-blue-300 text-black animate-pulse shadow-lg shadow-blue-400/50"
                : "bg-gray-700 text-gray-500 cursor-not-allowed"
            }`}
          >
            REPORT
          </button>
        )}

        {/* USE button (crewmate near task) */}
        {myRole === "crewmate" && !isFrozen && (
          <button
            onClick={handleUse}
            disabled={!nearTaskId}
            className={`w-20 h-20 rounded-full font-bold text-xl transition-all ${
              nearTaskId
                ? "bg-yellow-500 hover:bg-yellow-400 text-black animate-pulse shadow-lg shadow-yellow-500/50"
                : "bg-gray-700 text-gray-500 cursor-not-allowed"
            }`}
          >
            USE
          </button>
        )}
      </div>

      {/* EMERGENCY button (only in cafeteria) */}
      {!isFrozen && (
        <button
          onClick={handleEmergency}
          disabled={!canEmergency}
          className={`absolute bottom-6 left-1/2 -translate-x-1/2 z-10 px-6 py-3 rounded-xl font-black text-base transition-all ${
            canEmergency
              ? "bg-orange-500 hover:bg-orange-400 text-white shadow-lg shadow-orange-500/50 animate-pulse"
              : "hidden"
          }`}
        >
          🚨 EMERGENCY
        </button>
      )}

      {/* Role indicator + leave button — top left */}
      <div className="absolute top-4 left-4 z-10 flex flex-col gap-2 items-start">
        <div className="px-4 py-2 rounded-xl bg-black/60 backdrop-blur">
          <span
            className={`font-bold text-lg ${
              myRole === "tagger" ? "text-red-400" : "text-blue-400"
            }`}
          >
            {myRole === "tagger" ? "Tagger" : "Crewmate"}
          </span>
          {isFrozen && (
            <span className="ml-2 text-blue-300 text-sm">❄️ Ghost</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => {
              if (confirm("Leave the game?")) {
                send("leave_room");
                useGameStore.getState().leaveRoom();
              }
            }}
            className="px-3 py-1.5 rounded-lg bg-black/40 hover:bg-red-900/60 text-gray-400 hover:text-white text-xs transition-colors"
          >
            Leave Game
          </button>
          <MuteButton />
        </div>
      </div>
    </>
  );
}
