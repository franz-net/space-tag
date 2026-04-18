import { create } from "zustand";
import type {
  Player,
  Role,
  RoomStatePayload,
  RoomSettings,
  MapDataPayload,
  PlayerTaskInfo,
  TaskDataPayload,
  GameOverPayload,
  MeetingStartPayload,
  ChatMessagePayload,
  MeetingEndPayload,
  SabotageStartPayload,
  SabotageType,
  FixStation,
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
  settings: RoomSettings;

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

  // Sabotage
  activeSabotage: SabotageType | null;
  sabotageStations: FixStation[];
  sabotageEnd: number; // timestamp when sabotage expires
  sabotageCooldownEnd: number;
  meltdownUsed: boolean;
  meltdownTimer: number; // seconds remaining (from server)

  // Meeting
  meeting: MeetingStartPayload | null;
  meetingPhase: "discussion" | "voting" | null;
  meetingPhaseEnd: number; // timestamp
  chatMessages: ChatMessagePayload[];
  myVote: string | null; // empty string = skip
  votedPlayers: Set<string>; // IDs of players who have cast a vote
  meetingResult: MeetingEndPayload | null;

  // Game over
  gameOver: GameOverPayload | null;

  // Error
  error: string | null;
  clearError: () => void;

  // Sabotage actions
  setSabotage: (payload: SabotageStartPayload) => void;
  clearSabotage: () => void;
  setSabotageCooldown: (seconds: number) => void;
  setMeltdownTimer: (seconds: number) => void;

  // Actions
  setRoomState: (payload: RoomStatePayload) => void;
  setGameStarted: (role: Role, myId: string) => void;
  setMapData: (data: MapDataPayload["map"]) => void;
  returnToLobby: () => void;
  setTaskProgress: (progress: number, tasks: PlayerTaskInfo[]) => void;
  setActiveTask: (task: TaskDataPayload | null) => void;
  setFrozen: (ids: string[]) => void;
  freezePlayer: (id: string) => void;
  setCooldown: (seconds: number) => void;
  startMeeting: (payload: MeetingStartPayload) => void;
  setMeetingPhase: (phase: "voting", duration: number) => void;
  addChatMessage: (msg: ChatMessagePayload) => void;
  setMyVote: (targetId: string) => void;
  recordVoteCast: (voterId: string) => void;
  endMeeting: (result: MeetingEndPayload) => void;
  clearMeeting: () => void;
  setGameOver: (payload: GameOverPayload) => void;
  leaveRoom: () => void;
  setError: (msg: string) => void;
}

export const useGameStore = create<GameState>((set) => ({
  playerName:
    typeof window !== "undefined"
      ? localStorage.getItem("spacetag.name") ?? ""
      : "",
  setPlayerName: (name) => {
    if (typeof window !== "undefined") {
      localStorage.setItem("spacetag.name", name);
    }
    set({ playerName: name });
  },

  screen: "home",

  roomCode: null,
  roomState: null,
  players: [],
  hostId: null,
  myId: null,
  settings: { tasksPerPlayer: 4, discussionTime: 30, votingTime: 20, tagCooldown: 25 },

  myRole: null,
  mapData: null,

  myTasks: [],
  taskProgress: 0,
  activeTask: null,

  frozenIds: new Set(),
  tagCooldownEnd: 0,
  usedEmergency: false,

  activeSabotage: null,
  sabotageStations: [],
  sabotageEnd: 0,
  sabotageCooldownEnd: 0,
  meltdownUsed: false,
  meltdownTimer: 0,

  meeting: null,
  meetingPhase: null,
  meetingPhaseEnd: 0,
  chatMessages: [],
  myVote: null,
  votedPlayers: new Set(),
  meetingResult: null,

  gameOver: null,

  error: null,
  clearError: () => set({ error: null }),

  setSabotage: (payload) =>
    set({
      activeSabotage: payload.type,
      sabotageStations: payload.stations,
      sabotageEnd: Date.now() + payload.duration * 1000,
      meltdownUsed:
        payload.type === "meltdown" ? true : useGameStore.getState().meltdownUsed,
    }),

  clearSabotage: () =>
    set({
      activeSabotage: null,
      sabotageStations: [],
      sabotageEnd: 0,
      meltdownTimer: 0,
    }),

  setSabotageCooldown: (seconds) =>
    set({ sabotageCooldownEnd: Date.now() + seconds * 1000 }),

  setMeltdownTimer: (seconds) => set({ meltdownTimer: seconds }),

  setRoomState: (payload) =>
    set((state) => ({
      roomCode: payload.code,
      roomState: payload.state,
      players: payload.players,
      hostId: payload.hostId,
      myId: payload.you,
      settings: payload.settings,
      // Only auto-transition to lobby on initial join (from home).
      // Don't yank players out of game/gameover when room_state arrives mid-game.
      screen: state.screen === "home" ? "lobby" : state.screen,
      error: null,
    })),

  setGameStarted: (role, myId) =>
    set({
      myRole: role,
      myId: myId,
      roomState: "playing",
      screen: "game",
    }),

  setMapData: (data) => set({ mapData: data }),

  // Return to the lobby (without leaving the room) — used for Play Again
  returnToLobby: () =>
    set({
      screen: "lobby",
      myRole: null,
      mapData: null,
      myTasks: [],
      taskProgress: 0,
      activeTask: null,
      frozenIds: new Set(),
      tagCooldownEnd: 0,
      usedEmergency: false,
      activeSabotage: null,
      sabotageStations: [],
      sabotageEnd: 0,
      sabotageCooldownEnd: 0,
      meltdownUsed: false,
      meltdownTimer: 0,
      meeting: null,
      meetingPhase: null,
      meetingPhaseEnd: 0,
      chatMessages: [],
      myVote: null,
      votedPlayers: new Set(),
      meetingResult: null,
      gameOver: null,
    }),

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
      votedPlayers: new Set(),
      meetingResult: null,
      activeTask: null, // close any active task
    }),

  setMeetingPhase: (phase, duration) =>
    set({
      meetingPhase: phase,
      meetingPhaseEnd: Date.now() + duration * 1000,
    }),

  addChatMessage: (msg) =>
    set((state) => ({ chatMessages: [...state.chatMessages, msg] })),

  setMyVote: (targetId) => set({ myVote: targetId }),

  recordVoteCast: (voterId) =>
    set((state) => {
      const next = new Set(state.votedPlayers);
      next.add(voterId);
      return { votedPlayers: next };
    }),

  endMeeting: (result) =>
    set({ meetingResult: result, meetingPhase: null }),

  clearMeeting: () =>
    set({
      meeting: null,
      meetingPhase: null,
      chatMessages: [],
      myVote: null,
      votedPlayers: new Set(),
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
      activeSabotage: null,
      sabotageStations: [],
      sabotageEnd: 0,
      sabotageCooldownEnd: 0,
      meltdownUsed: false,
      meltdownTimer: 0,
      meeting: null,
      meetingPhase: null,
      chatMessages: [],
      myVote: null,
      votedPlayers: new Set(),
      meetingResult: null,
      gameOver: null,
      screen: "home",
    }),

  setError: (msg) => set({ error: msg }),
}));
