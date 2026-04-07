"use client";

import { useEffect, useRef, useCallback, useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { Engine } from "@/game/Engine";
import HUD from "./HUD";
import TaskOverlay from "./TaskOverlay";
import GameOverScreen from "./GameOverScreen";
import MeetingScreen from "./MeetingScreen";
import Joystick from "./Joystick";
import type { MsgType, PositionsPayload } from "@/lib/protocol";

interface GameScreenProps {
  send: (type: MsgType, payload?: unknown) => void;
  positionsRef: React.RefObject<PositionsPayload | null>;
}

export default function GameScreen({ send, positionsRef }: GameScreenProps) {
  const { players, myId, mapData, myTasks, gameOver, myRole, meeting } =
    useGameStore();
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const engineRef = useRef<Engine | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [nearTaskId, setNearTaskId] = useState<string | null>(null);
  const [nearTagTargetId, setNearTagTargetId] = useState<string | null>(null);
  const [nearBodyId, setNearBodyId] = useState<string | null>(null);
  const [inCafeteria, setInCafeteria] = useState(false);

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

  // Initialize engine
  useEffect(() => {
    if (!canvasRef.current || !mapData || !myId) return;

    const canvas = canvasRef.current;
    const container = containerRef.current!;
    const engine = new Engine(onMove);

    const playerInfos = players.map((p) => ({
      id: p.id,
      color: p.color,
      name: p.name,
    }));

    let cancelled = false;
    engine
      .init(canvas, container.clientWidth, container.clientHeight)
      .then(() => {
        if (cancelled) return;
        engine.setupMap(mapData, playerInfos, myId);
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

    const handleResize = () => {
      if (engineRef.current && containerRef.current) {
        engineRef.current.resize(
          containerRef.current.clientWidth,
          containerRef.current.clientHeight
        );
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      cancelled = true;
      window.removeEventListener("resize", handleResize);
      engine.destroy();
      engineRef.current = null;
    };
  }, [mapData, myId, players, onMove]);

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
        inCafeteria={inCafeteria}
      />

      {/* Joystick — only on small screens */}
      <Joystick onMove={handleJoystickMove} />

      {/* Controls hint — only on larger screens */}
      <div className="hidden md:block absolute bottom-4 left-4 z-10 px-3 py-1.5 rounded-lg bg-black/40 text-gray-400 text-sm">
        WASD or Arrow keys to move
      </div>

      <canvas ref={canvasRef} className="w-full h-full block" />

      <TaskOverlay send={send} />
      {meeting && <MeetingScreen send={send} />}
      {gameOver && <GameOverScreen send={send} />}
    </div>
  );
}
