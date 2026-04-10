package main

import "encoding/json"

type MsgType string

const (
	MsgCreateRoom   MsgType = "create_room"
	MsgJoinRoom     MsgType = "join_room"
	MsgLeaveRoom    MsgType = "leave_room"
	MsgRoomState    MsgType = "room_state"
	MsgStartGame    MsgType = "start_game"
	MsgGameStarted  MsgType = "game_started"
	MsgPlayerJoined MsgType = "player_joined"
	MsgPlayerLeft   MsgType = "player_left"
	MsgError        MsgType = "error"
	MsgAddAI        MsgType = "add_ai"
	MsgRemoveAI     MsgType = "remove_ai"
	MsgMove         MsgType = "move"
	MsgPositions    MsgType = "positions"
	MsgMapData      MsgType = "map_data"
	MsgTaskStart    MsgType = "task_start"
	MsgTaskData     MsgType = "task_data"
	MsgTaskComplete MsgType = "task_complete"
	MsgTaskProgress MsgType = "task_progress"
	MsgGameOver     MsgType = "game_over"
	MsgTagPlayer    MsgType = "tag_player"
	MsgPlayerFrozen MsgType = "player_frozen"
	MsgReportBody   MsgType = "report_body"
	MsgEmergency    MsgType = "emergency"
	MsgMeetingStart MsgType = "meeting_start"
	MsgChatMessage  MsgType = "chat_message"
	MsgCastVote     MsgType = "cast_vote"
	MsgVoteCast     MsgType = "vote_cast"
	MsgMeetingEnd   MsgType = "meeting_end"
	MsgCooldown       MsgType = "cooldown"
	MsgRoomSettings   MsgType = "room_settings"   // client→server: host updates settings
	MsgSabotage       MsgType = "sabotage"       // client→server: tagger activates
	MsgSabotageStart  MsgType = "sabotage_start"  // server→all: sabotage began
	MsgSabotageEnd    MsgType = "sabotage_end"    // server→all: sabotage cleared
	MsgSabotageFix    MsgType = "sabotage_fix"    // client→server: crewmate fixes
)

type Envelope struct {
	Type    MsgType         `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type CreateRoomPayload struct {
	PlayerName string `json:"playerName"`
}

type JoinRoomPayload struct {
	RoomCode   string `json:"roomCode"`
	PlayerName string `json:"playerName"`
}

type RoomStatePayload struct {
	Code     string       `json:"code"`
	State    RoomPhase    `json:"state"`
	Players  []Player     `json:"players"`
	HostID   string       `json:"hostId"`
	You      string       `json:"you"`
	Settings RoomSettings `json:"settings"`
}

type GameStartedPayload struct {
	Role Role   `json:"role"`
	You  string `json:"you"`
}

type PlayerEventPayload struct {
	Player Player `json:"player"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

// Task payloads

type PlayerTaskInfo struct {
	StationID string   `json:"stationId"`
	Type      TaskType `json:"type"`
	RoomID    string   `json:"roomId"`
	Position  Vec2     `json:"position"`
	Completed bool     `json:"completed"`
}

type TaskStartPayload struct {
	StationID string `json:"stationId"`
}

type TaskDataPayload struct {
	StationID string      `json:"stationId"`
	Type      TaskType    `json:"type"`
	Params    interface{} `json:"params"` // task-specific random params
}

type TaskCompletePayload struct {
	StationID string `json:"stationId"`
}

type TaskProgressPayload struct {
	Progress float64          `json:"progress"` // 0.0 to 1.0
	Tasks    []PlayerTaskInfo `json:"tasks"`    // updated task list for this player
}

type GameOverPayload struct {
	Winner string   `json:"winner"` // "crew" or "tagger"
	Roles  []PlayerRoleInfo `json:"roles"`
}

type PlayerRoleInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// Tagger / voting payloads

type TagPlayerPayload struct {
	TargetID string `json:"targetId"`
}

type PlayerFrozenPayload struct {
	PlayerID string `json:"playerId"`
	Position Vec2   `json:"position"`
}

type ReportBodyPayload struct {
	BodyID string `json:"bodyId"`
}

type MeetingStartPayload struct {
	CallerID         string  `json:"callerId"`
	Reason           string  `json:"reason"` // "body" or "emergency"
	BodyID           string  `json:"bodyId,omitempty"`
	DiscussionTime   float64 `json:"discussionTime"` // seconds
	VotingTime       float64 `json:"votingTime"`     // seconds
	AlivePlayers     []string `json:"alivePlayers"`
}

type ChatMessagePayload struct {
	SenderID  string `json:"senderId"`
	MessageID string `json:"messageId"`
}

type CastVotePayload struct {
	TargetID string `json:"targetId"` // empty string = skip
}

type VoteCastPayload struct {
	VoterID string `json:"voterId"` // who voted (target stays secret)
}

type MeetingEndPayload struct {
	Votes      map[string]string `json:"votes"`     // voterID -> targetID ("" = skip)
	EjectedID  string            `json:"ejectedId"` // empty if no one ejected
	WasTagger  bool              `json:"wasTagger"` // true if ejected was the tagger
}

type CooldownPayload struct {
	Seconds float64 `json:"seconds"`
}
