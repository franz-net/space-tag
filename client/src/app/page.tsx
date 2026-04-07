"use client";

import { useCallback, useRef } from "react";
import { useWebSocket } from "@/hooks/useWebSocket";
import { useGameStore } from "@/stores/gameStore";
import HomeScreen from "@/components/HomeScreen";
import Lobby from "@/components/Lobby";
import GameScreen from "@/components/GameScreen";
import type {
  MsgType,
  RoomStatePayload,
  GameStartedPayload,
  MapDataPayload,
  PositionsPayload,
  TaskDataPayload,
  TaskProgressPayload,
  GameOverPayload,
  PlayerFrozenPayload,
  MeetingStartPayload,
  ChatMessagePayload,
  MeetingEndPayload,
  CooldownPayload,
  ErrorPayload,
} from "@/lib/protocol";

export default function Home() {
  const {
    screen,
    setRoomState,
    setGameStarted,
    setMapData,
    setTaskProgress,
    setActiveTask,
    setGameOver,
    freezePlayer,
    setCooldown,
    startMeeting,
    addChatMessage,
    endMeeting,
    clearMeeting,
    setError,
  } = useGameStore();

  // Use a ref for positions to avoid re-renders on every tick (20Hz)
  const positionsRef = useRef<PositionsPayload | null>(null);

  const onMessage = useCallback(
    (type: MsgType, payload: unknown) => {
      switch (type) {
        case "room_state": {
          setRoomState(payload as RoomStatePayload);
          break;
        }
        case "map_data": {
          setMapData((payload as MapDataPayload).map);
          break;
        }
        case "game_started": {
          const gs = payload as GameStartedPayload;
          setGameStarted(gs.role, gs.you);
          break;
        }
        case "positions": {
          positionsRef.current = payload as PositionsPayload;
          break;
        }
        case "task_data": {
          setActiveTask(payload as TaskDataPayload);
          break;
        }
        case "task_progress": {
          const tp = payload as TaskProgressPayload;
          setTaskProgress(tp.progress, tp.tasks);
          break;
        }
        case "player_frozen": {
          const f = payload as PlayerFrozenPayload;
          freezePlayer(f.playerId);
          break;
        }
        case "cooldown": {
          const c = payload as CooldownPayload;
          setCooldown(c.seconds);
          break;
        }
        case "meeting_start": {
          startMeeting(payload as MeetingStartPayload);
          break;
        }
        case "chat_message": {
          addChatMessage(payload as ChatMessagePayload);
          break;
        }
        case "meeting_end": {
          endMeeting(payload as MeetingEndPayload);
          // Clear meeting after a short reveal delay
          setTimeout(() => {
            clearMeeting();
          }, 4000);
          break;
        }
        case "game_over": {
          setGameOver(payload as GameOverPayload);
          break;
        }
        case "error": {
          setError((payload as ErrorPayload).message);
          break;
        }
      }
    },
    [
      setRoomState,
      setGameStarted,
      setMapData,
      setTaskProgress,
      setActiveTask,
      setGameOver,
      freezePlayer,
      setCooldown,
      startMeeting,
      addChatMessage,
      endMeeting,
      clearMeeting,
      setError,
    ]
  );

  const { send, connected } = useWebSocket(onMessage);

  return (
    <main className="flex-1 flex flex-col items-center justify-center bg-gray-950 px-4">
      {screen === "home" && <HomeScreen send={send} connected={connected} />}
      {screen === "lobby" && <Lobby send={send} connected={connected} />}
      {screen === "game" && (
        <GameScreen send={send} positionsRef={positionsRef} />
      )}
    </main>
  );
}
