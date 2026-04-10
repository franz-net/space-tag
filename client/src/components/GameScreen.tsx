"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { Engine } from "@/game/Engine";
import HUD from "./HUD";
import TaskOverlay from "./TaskOverlay";
import GameOverScreen from "./GameOverScreen";
import MeetingScreen from "./MeetingScreen";
import Joystick from "./Joystick";
import CountdownOverlay from "./CountdownOverlay";
import { useIsTouch } from "@/hooks/useIsTouch";
import type { MsgType, PositionsPayload } from "@/lib/protocol";

interface GameScreenProps {
  send: (type: MsgType, payload?: unknown) => void;
  positionsRef: React.RefObject<PositionsPayload | null>;
}

export default function GameScreen({ send, positionsRef }: GameScreenProps) {
  const { myId, mapData, myTasks, gameOver, myRole, meeting } =
    useGameStore();
  const isTouch = useIsTouch();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const engineRef = useRef<Engine | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [nearTaskId, setNearTaskId] = useState<string | null>(null);
  const [nearTagTargetId, setNearTagTargetId] = useState<string | null>(null);
  const [nearBodyId, setNearBodyId] = useState<string | null>(null);
  const [nearFixStationId, setNearFixStationId] = useState<string | null>(null);
  const [inCafeteria, setInCafeteria] = useState(false);
  // Show 3-2-1 countdown once at the start of each game (resets when
  // mapData clears on returnToLobby).
  const [showCountdown, setShowCountdown] = useState(false);
  useEffect(() => {
    if (mapData) setShowCountdown(true);
    else setShowCountdown(false);
  }, [mapData]);

  const onMove = useCallback(
    (dx: number, dy: number) => {
      send("move", { dx, dy });
    },
    [send]
  );

  const handleJoystickMove = useCallback((x: number, y: number) => {
    if (engineRef.current) {
      engineRef.current.input.setJoystickDirection(x, y);
    }
  }, []);

  // Initialize engine ONCE per game (not per `players` change!)
  //
  // Critical: do NOT include `players` in this dep array. Every time the
  // server broadcasts room_state (e.g. on game over → resetRoomToLobby),
  // `players` becomes a new array reference. If `players` were a dep, the
  // effect would re-run, destroying the PixiJS app + WebGL context, and
  // the new app.init() on the same canvas would silently fail to start
  // its ticker — leading to a frozen-looking screen with no errors.
  //
  // Player info is read from the store at init time. Other state (role,
  // tasks, frozen) is propagated by the dedicated effects below.
  useEffect(() => {
    if (!canvasRef.current || !mapData || !myId) return;

    const canvas = canvasRef.current;
    const container = containerRef.current!;
    const engine = new Engine(onMove);

    const initialPlayers = useGameStore.getState().players.map((p) => ({
      id: p.id,
      color: p.color,
      name: p.name,
    }));

    let cancelled = false;
    engine
      .init(canvas, container.clientWidth, container.clientHeight)
      .then(() => {
        if (cancelled) return;
        engine.setupMap(mapData, initialPlayers, myId);
        const state = useGameStore.getState();
        if (state.myTasks.length > 0) {
          engine.setupTasks(state.myTasks);
        }
        if (state.myRole) {
          engine.setMyRole(state.myRole);
        }
        engine.setFrozen(state.frozenIds);
        engine.markReady();
        engineRef.current = engine;
      });

    // Resize the renderer whenever the visible viewport changes. We listen
    // to multiple events because:
    //   - `resize` covers desktop window resizes and most mobile cases
    //   - `orientationchange` fires before iOS updates dimensions, so we
    //     re-resize again on a short delay to catch the new viewport
    //   - `visualViewport.resize` catches iOS keyboard show/hide and
    //     pinch-zoom, which `resize` misses on iPad
    const doResize = () => {
      if (engineRef.current && containerRef.current) {
        engineRef.current.resize(
          containerRef.current.clientWidth,
          containerRef.current.clientHeight
        );
      }
    };
    const handleOrientationChange = () => {
      // iOS reports the OLD dimensions during orientationchange. Re-resize
      // a few times after to make sure we land on the correct viewport.
      doResize();
      setTimeout(doResize, 100);
      setTimeout(doResize, 400);
    };
    window.addEventListener("resize", doResize);
    window.addEventListener("orientationchange", handleOrientationChange);
    const vv = window.visualViewport;
    vv?.addEventListener("resize", doResize);

    return () => {
      cancelled = true;
      window.removeEventListener("resize", doResize);
      window.removeEventListener("orientationchange", handleOrientationChange);
      vv?.removeEventListener("resize", doResize);
      engine.destroy();
      engineRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mapData, myId, onMove]);

  // Update role on engine when it changes
  useEffect(() => {
    if (engineRef.current && myRole) {
      engineRef.current.setMyRole(myRole);
    }
  }, [myRole]);

  // Setup task stations when tasks arrive
  useEffect(() => {
    if (engineRef.current && myTasks.length > 0) {
      engineRef.current.setupTasks(myTasks);
    }
  }, [myTasks.length]);

  // Update task completion visuals
  useEffect(() => {
    if (engineRef.current && myTasks.length > 0) {
      engineRef.current.updateTasks(myTasks);
    }
  }, [myTasks]);

  // Poll for position updates and nearby state.
  // Errors here MUST NOT kill the rAF loop — wrap each call so the next
  // frame still runs and the game doesn't appear frozen.
  useEffect(() => {
    let animId: number;
    const poll = () => {
      try {
        if (engineRef.current && positionsRef.current) {
          engineRef.current.updatePositions(positionsRef.current.positions);
          engineRef.current.setBodies(positionsRef.current.bodies || {});
          engineRef.current.setFrozen(
            new Set(positionsRef.current.frozen || [])
          );
          // Sync sabotage state from positions payload to engine + store
          const sabType = positionsRef.current.sabotage || null;
          const store = useGameStore.getState();
          const fixPositions = store.sabotageStations.map((s) => ({
            id: s.id,
            x: s.position.x,
            y: s.position.y,
          }));
          engineRef.current.setSabotage(sabType, fixPositions);
          // Sync meltdown timer
          if (sabType === "meltdown") {
            store.setMeltdownTimer(positionsRef.current.meltdownTimer);
          }

          // Sync to store too (for HUD) only when the set actually changes
          const currentFrozen = useGameStore.getState().frozenIds;
          const newFrozen = positionsRef.current.frozen || [];
          if (
            newFrozen.length !== currentFrozen.size ||
            newFrozen.some((id) => !currentFrozen.has(id))
          ) {
            useGameStore.getState().setFrozen(newFrozen);
          }
        }
        if (engineRef.current) {
          setNearTaskId(engineRef.current.nearTaskId);
          setNearTagTargetId(engineRef.current.nearTagTargetId);
          setNearBodyId(engineRef.current.nearBodyId);
          setNearFixStationId(engineRef.current.nearFixStationId);
          setInCafeteria(engineRef.current.inCafeteria);
        }
      } catch (err) {
        console.error("Game poll error:", err);
      }
      animId = requestAnimationFrame(poll);
    };
    animId = requestAnimationFrame(poll);
    return () => cancelAnimationFrame(animId);
  }, [positionsRef]);

  return (
    <div ref={containerRef} className="w-full h-full relative">
      <HUD
        send={send}
        nearTaskId={nearTaskId}
        nearTagTargetId={nearTagTargetId}
        nearBodyId={nearBodyId}
        nearFixStationId={nearFixStationId}
        inCafeteria={inCafeteria}
      />

      {/* Joystick — only on small screens */}
      <Joystick onMove={handleJoystickMove} />

      {/* Controls hint — only for keyboard users (not touch devices) */}
      {!isTouch && (
        <div className="absolute bottom-4 left-4 z-10 px-3 py-1.5 rounded-lg bg-black/40 text-gray-400 text-sm">
          WASD or Arrow keys to move
        </div>
      )}

      <canvas ref={canvasRef} className="w-full h-full block" />

      <TaskOverlay send={send} />
      {showCountdown && !meeting && !gameOver && (
        <CountdownOverlay onDone={() => setShowCountdown(false)} />
      )}
      {meeting && <MeetingScreen send={send} />}
      {gameOver && <GameOverScreen send={send} />}
    </div>
  );
}
