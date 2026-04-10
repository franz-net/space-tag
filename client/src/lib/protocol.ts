// Message types — mirrors server/messages.go
export type MsgType =
  | "create_room"
  | "join_room"
  | "leave_room"
  | "room_state"
  | "start_game"
  | "game_started"
  | "player_joined"
  | "player_left"
  | "add_ai"
  | "remove_ai"
  | "move"
  | "positions"
  | "map_data"
  | "task_start"
  | "task_data"
  | "task_complete"
  | "task_progress"
  | "game_over"
  | "tag_player"
  | "player_frozen"
  | "report_body"
  | "emergency"
  | "meeting_start"
  | "chat_message"
  | "cast_vote"
  | "vote_cast"
  | "meeting_end"
  | "cooldown"
  | "room_settings"
  | "sabotage"
  | "sabotage_start"
  | "sabotage_end"
  | "sabotage_fix"
  | "error";

export interface Envelope {
  type: MsgType;
  payload: unknown;
}

export type PlayerColor =
  | "red"
  | "blue"
  | "green"
  | "yellow"
  | "purple"
  | "orange";

export type Role = "crewmate" | "tagger";

export interface Player {
  id: string;
  name: string;
  color: PlayerColor;
  isHost: boolean;
  isAI: boolean;
  isAlive: boolean;
}

export interface RoomSettings {
  tasksPerPlayer: number;
  discussionTime: number;
  votingTime: number;
  tagCooldown: number;
}

export interface RoomStatePayload {
  code: string;
  state: "lobby" | "playing" | "voting" | "ended";
  players: Player[];
  hostId: string;
  you: string;
  settings: RoomSettings;
}

export interface GameStartedPayload {
  role: Role;
  you: string;
}

export interface ErrorPayload {
  message: string;
}

export interface PositionsPayload {
  positions: Record<string, { x: number; y: number }>;
  frozen: string[];
  bodies: Record<string, { x: number; y: number }>;
  sabotage: string; // "" or "lights_out" | "comms_down" | "meltdown"
  meltdownTimer: number; // seconds remaining (0 if not meltdown)
}

export interface MapDataPayload {
  map: {
    rooms: {
      id: string;
      label: string;
      bounds: { x: number; y: number; w: number; h: number };
      color: string;
    }[];
    hallways: {
      bounds: { x: number; y: number; w: number; h: number };
    }[];
    walls: { x: number; y: number; w: number; h: number }[] | null;
    spawnPos: { x: number; y: number };
    width: number;
    height: number;
  };
}

// Task types
export type TaskType = "tap_targets" | "connect_wires" | "match_colors" | "simon_says";

export interface PlayerTaskInfo {
  stationId: string;
  type: TaskType;
  roomId: string;
  position: { x: number; y: number };
  completed: boolean;
}

export interface TaskDataPayload {
  stationId: string;
  type: TaskType;
  params: Record<string, unknown>;
}

export interface TaskProgressPayload {
  progress: number;
  tasks: PlayerTaskInfo[];
}

export interface GameOverPayload {
  winner: "crew" | "tagger";
  roles: { id: string; name: string; role: Role }[];
}

// Tagger / voting payloads
export interface PlayerFrozenPayload {
  playerId: string;
  position: { x: number; y: number };
}

export interface MeetingStartPayload {
  callerId: string;
  reason: "body" | "emergency";
  bodyId?: string;
  discussionTime: number;
  votingTime: number;
  alivePlayers: string[];
}

export interface ChatMessagePayload {
  senderId: string;
  messageId: string;
}

export interface VoteCastPayload {
  voterId: string;
}

export interface MeetingEndPayload {
  votes: Record<string, string>;
  ejectedId: string;
  wasTagger: boolean;
}

export interface CooldownPayload {
  seconds: number;
}

// Sabotage types
export type SabotageType = "lights_out" | "comms_down" | "meltdown";

export interface FixStation {
  id: string;
  roomId: string;
  position: { x: number; y: number };
}

export interface SabotageStartPayload {
  type: SabotageType;
  duration: number;
  stations: FixStation[];
}

// Pre-defined chat messages with display info
export interface QuickMessage {
  id: string;
  icon: string;
  text: string;
  category: "accuse" | "defend" | "info" | "location" | "vote";
}

export const QUICK_MESSAGES: QuickMessage[] = [
  // Accusations — answer "Who?"
  { id: "sus_red", icon: "🔴", text: "Red is sus", category: "accuse" },
  { id: "sus_blue", icon: "🔵", text: "Blue is sus", category: "accuse" },
  { id: "sus_green", icon: "🟢", text: "Green is sus", category: "accuse" },
  { id: "sus_yellow", icon: "🟡", text: "Yellow is sus", category: "accuse" },
  { id: "sus_purple", icon: "🟣", text: "Purple is sus", category: "accuse" },
  { id: "sus_orange", icon: "🟠", text: "Orange is sus", category: "accuse" },
  // Defense
  { id: "trust_me", icon: "💚", text: "Trust me", category: "defend" },
  { id: "not_me", icon: "🙅", text: "Not me!", category: "defend" },
  { id: "doing_task", icon: "🔧", text: "Doing tasks", category: "defend" },
  { id: "with_me", icon: "🤝", text: "With me", category: "defend" },
  // Info
  { id: "i_saw", icon: "👁️", text: "I saw something", category: "info" },
  { id: "where", icon: "❓", text: "Where?", category: "info" },
  { id: "idk", icon: "🤷", text: "I don't know", category: "info" },
  // Locations — answer "Where?"
  { id: "loc_cafeteria", icon: "🍽️", text: "Cafeteria", category: "location" },
  { id: "loc_medbay", icon: "🏥", text: "Medbay", category: "location" },
  { id: "loc_navigation", icon: "🧭", text: "Navigation", category: "location" },
  { id: "loc_engine", icon: "⚙️", text: "Engine", category: "location" },
  { id: "loc_storage", icon: "📦", text: "Storage", category: "location" },
  { id: "loc_reactor", icon: "☢️", text: "Reactor", category: "location" },
  // Vote
  { id: "vote_skip", icon: "⏭️", text: "Skip vote", category: "vote" },
];

// Color display values
export const COLOR_HEX: Record<PlayerColor, string> = {
  red: "#EF4444",
  blue: "#3B82F6",
  green: "#22C55E",
  yellow: "#EAB308",
  purple: "#A855F7",
  orange: "#F97316",
};

export const COLOR_NAMES: Record<PlayerColor, string> = {
  red: "Red",
  blue: "Blue",
  green: "Green",
  yellow: "Yellow",
  purple: "Purple",
  orange: "Orange",
};
