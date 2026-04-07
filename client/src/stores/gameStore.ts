import { create } from "zustand";
import type {
  Player,
  Role,
  RoomStatePayload,
  MapDataPayload,
  PlayerTaskInfo,
  TaskDataPayload,
  GameOverPayload,
  MeetingStartPayload,
  ChatMessagePayload,
  MeetingEndPayload,
} from "@/lib/protocol";

export type Screen = "home" | "lobby" | "game";

interface GameState {
  // Connection
  playerName: string;
  setPlayerName: (name: string) => void;

  // Screen
  screen: Screen;

  // Room
  roomCode: string | null;
  roomState: "lobby" | "playing" | "voting" | "ended" | null;
  players: Player[];
  hostId: string | null;
  myId: string | null;

  // Game
  myRole: Role | null;
  mapData: MapDataPayload["map"] | null;

  // Tasks
  myTasks: PlayerTaskInfo[];
  taskProgress: number;
  activeTask: TaskDataPayload | null;

  // Tagger / freeze
  frozenIds: Set<string>;
  tagCooldownEnd: number; // timestamp when cooldown ends
  usedEmergency: boolean;

  // Meeting
  meeting: MeetingStartPayload | null;
  meetingPhase: "discussion" | "voting" | null;
  meetingPhaseEnd: number; // timestamp
  chatMessages: ChatMessagePayload[];
  myVote: string | null; // empty string = skip
  meetingResult: MeetingEndPayload | null;

  // Game over
  gameOver: GameOverPayload | null;

  // Error
  error: string | null;
  clearError: () => void;

  // Actions
  setRoomState: (payload: RoomStatePayload) => void;
  setGameStarted: (role: Role, myId: string) => void;
  setMapData: (data: MapDataPayload["map"]) => void;
  setTaskProgress: (progress: number, tasks: PlayerTaskInfo[]) => void;
  setActiveTask: (task: TaskDataPayload | null) => void;
  setFrozen: (ids: string[]) => void;
  freezePlayer: (id: string) => void;
  setCooldown: (seconds: number) => void;
  startMeeting: (payload: MeetingStartPayload) => void;
  addChatMessage: (msg: ChatMessagePayload) => void;
  setMyVote: (targetId: string) => void;
  endMeeting: (result: MeetingEndPayload) => void;
  clearMeeting: () => void;
  setGameOver: (payload: GameOverPayload) => void;
  leaveRoom: () => void;
  setError: (msg: string) => void;
}

export const useGameStore = create<GameState>((set) => ({
  playerName: "",
  setPlayerName: (name) => set({ playerName: name }),

  screen: "home",

  roomCode: null,
  roomState: null,
  players: [],
  hostId: null,
  myId: null,

  myRole: null,
  mapData: null,

  myTasks: [],
  taskProgress: 0,
  activeTask: null,

  frozenIds: new Set(),
  tagCooldownEnd: 0,
  usedEmergency: false,

  meeting: null,
  meetingPhase: null,
  meetingPhaseEnd: 0,
  chatMessages: [],
  myVote: null,
  meetingResult: null,

  gameOver: null,

  error: null,
  clearError: () => set({ error: null }),

  setRoomState: (payload) =>
    set({
      roomCode: payload.code,
      roomState: payload.state,
      players: payload.players,
      hostId: payload.hostId,
      myId: payload.you,
      screen: "lobby",
      error: null,
    }),

  setGameStarted: (role, myId) =>
    set({
      myRole: role,
      myId: myId,
      roomState: "playing",
      screen: "game",
    }),

  setMapData: (data) => set({ mapData: data }),

  setTaskProgress: (progress, tasks) =>
    set({ taskProgress: progress, myTasks: tasks }),

  setActiveTask: (task) => set({ activeTask: task }),

  setFrozen: (ids) => set({ frozenIds: new Set(ids) }),

  freezePlayer: (id) =>
    set((state) => {
      const newSet = new Set(state.frozenIds);
      newSet.add(id);
      return { frozenIds: newSet };
    }),

  setCooldown: (seconds) =>
    set({ tagCooldownEnd: Date.now() + seconds * 1000 }),

  startMeeting: (payload) =>
    set({
      meeting: payload,
      meetingPhase: "discussion",
      meetingPhaseEnd: Date.now() + payload.discussionTime * 1000,
      chatMessages: [],
      myVote: null,
      meetingResult: null,
      activeTask: null, // close any active task
    }),

  addChatMessage: (msg) =>
    set((state) => ({ chatMessages: [...state.chatMessages, msg] })),

  setMyVote: (targetId) => set({ myVote: targetId }),

  endMeeting: (result) =>
    set({ meetingResult: result, meetingPhase: null }),

  clearMeeting: () =>
    set({
      meeting: null,
      meetingPhase: null,
      chatMessages: [],
      myVote: null,
      meetingResult: null,
    }),

  setGameOver: (payload) =>
    set({ gameOver: payload, roomState: "ended" }),

  leaveRoom: () =>
    set({
      roomCode: null,
      roomState: null,
      players: [],
      hostId: null,
      myId: null,
      myRole: null,
      mapData: null,
      myTasks: [],
      taskProgress: 0,
      activeTask: null,
      frozenIds: new Set(),
      tagCooldownEnd: 0,
      usedEmergency: false,
      meeting: null,
      meetingPhase: null,
      chatMessages: [],
      myVote: null,
      meetingResult: null,
      gameOver: null,
      screen: "home",
    }),

  setError: (msg) => set({ error: msg }),
}));
