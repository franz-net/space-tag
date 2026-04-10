"use client";

import { useCallback, useRef } from "react";
import { useWebSocket } from "@/hooks/useWebSocket";
import { useGameStore } from "@/stores/gameStore";
import { sounds } from "@/lib/sounds";
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
  VoteCastPayload,
  MeetingEndPayload,
  CooldownPayload,
  SabotageStartPayload,
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
    recordVoteCast,
    endMeeting,
    clearMeeting,
    setSabotage,
    clearSabotage,
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
          // Detect that one of MY tasks just completed (count went up)
          const prevDone = useGameStore
            .getState()
            .myTasks.filter((t) => t.completed).length;
          const newDone = tp.tasks.filter((t) => t.completed).length;
          if (newDone > prevDone) {
            sounds.taskComplete();
          }
          setTaskProgress(tp.progress, tp.tasks);
          break;
        }
        case "player_frozen": {
          const f = payload as PlayerFrozenPayload;
          sounds.freeze();
          freezePlayer(f.playerId);
          break;
        }
        case "cooldown": {
          const c = payload as CooldownPayload;
          setCooldown(c.seconds);
          break;
        }
        case "meeting_start": {
          sounds.meetingStart();
          startMeeting(payload as MeetingStartPayload);
          break;
        }
        case "chat_message": {
          addChatMessage(payload as ChatMessagePayload);
          break;
        }
        case "vote_cast": {
          const vc = payload as VoteCastPayload;
          recordVoteCast(vc.voterId);
          break;
        }
        case "meeting_end": {
          const me = payload as MeetingEndPayload;
          if (me.ejectedId) {
            sounds.ejection();
          }
          endMeeting(me);
          // Clear meeting after a short reveal delay
          setTimeout(() => {
            clearMeeting();
          }, 4000);
          break;
        }
        case "game_over": {
          const go = payload as GameOverPayload;
          // Win/lose tune from this player's perspective
          const myId = useGameStore.getState().myId;
          const myRole = useGameStore.getState().myRole;
          const me = go.roles.find((r) => r.id === myId);
          const iWon =
            (go.winner === "crew" && me?.role === "crewmate") ||
            (go.winner === "tagger" && me?.role === "tagger") ||
            (myRole === null && go.winner === "crew");
          if (iWon) {
            sounds.win();
          } else {
            sounds.lose();
          }
          setGameOver(go);
          break;
        }
        case "sabotage_start": {
          const s = payload as SabotageStartPayload;
          if (s.type === "meltdown") {
            sounds.meltdownAlarm();
          } else {
            sounds.sabotageStart();
          }
          setSabotage(s);
          break;
        }
        case "sabotage_end": {
          sounds.sabotageEnd();
          clearSabotage();
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
      recordVoteCast,
      endMeeting,
      clearMeeting,
      setSabotage,
      clearSabotage,
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
