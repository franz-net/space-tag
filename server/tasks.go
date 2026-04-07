package main

import (
	"math/rand"
)

type TaskType string

const (
	TaskTapTargets   TaskType = "tap_targets"
	TaskConnectWires TaskType = "connect_wires"
	TaskMatchColors  TaskType = "match_colors"
	TaskSimonSays    TaskType = "simon_says"
)

// TaskStation is a fixed location on the map where a task can be performed
type TaskStation struct {
	ID       string   `json:"id"`
	Type     TaskType `json:"type"`
	RoomID   string   `json:"roomId"`
	Position Vec2     `json:"position"`
}

// TaskAssignment tracks a player's assigned tasks
type TaskAssignment struct {
	StationID string `json:"stationId"`
	Completed bool   `json:"completed"`
}

// TasksState manages all task-related game state
type TasksState struct {
	Stations    []TaskStation              `json:"stations"`
	Assignments map[string][]TaskAssignment `json:"assignments"` // playerID -> tasks
	TotalTasks  int                        `json:"totalTasks"`   // total crew tasks needed
	DoneTasks   int                        `json:"doneTasks"`    // completed so far
}

// Pre-defined task stations placed around the ship
// Positions are inside the new room layout (rooms are 400x300)
func buildTaskStations() []TaskStation {
	return []TaskStation{
		// Medbay (x:100-500, y:100-400)
		{ID: "task-1", Type: TaskMatchColors, RoomID: "medbay", Position: Vec2{200, 250}},
		{ID: "task-2", Type: TaskSimonSays, RoomID: "medbay", Position: Vec2{400, 250}},
		// Cafeteria (x:900-1300, y:100-400)
		{ID: "task-3", Type: TaskTapTargets, RoomID: "cafeteria", Position: Vec2{1000, 250}},
		{ID: "task-4", Type: TaskConnectWires, RoomID: "cafeteria", Position: Vec2{1200, 250}},
		// Navigation (x:1700-2100, y:100-400)
		{ID: "task-5", Type: TaskTapTargets, RoomID: "navigation", Position: Vec2{1900, 250}},
		// Engine (x:100-500, y:900-1200)
		{ID: "task-6", Type: TaskConnectWires, RoomID: "engine", Position: Vec2{200, 1050}},
		{ID: "task-7", Type: TaskSimonSays, RoomID: "engine", Position: Vec2{400, 1050}},
		// Storage (x:900-1300, y:900-1200)
		{ID: "task-8", Type: TaskMatchColors, RoomID: "storage", Position: Vec2{1100, 1050}},
		// Reactor (x:1700-2100, y:900-1200)
		{ID: "task-9", Type: TaskTapTargets, RoomID: "reactor", Position: Vec2{1800, 1050}},
		{ID: "task-10", Type: TaskConnectWires, RoomID: "reactor", Position: Vec2{2000, 1050}},
	}
}

const TasksPerPlayer = 4
const TaskInteractionRange = 60.0

// InitTasks creates task state and assigns tasks to crewmates
func InitTasks(playerIDs []string, roles map[string]Role) *TasksState {
	stations := buildTaskStations()

	assignments := make(map[string][]TaskAssignment)
	totalTasks := 0

	for _, pid := range playerIDs {
		role := roles[pid]

		// Randomly pick TasksPerPlayer stations for this player
		perm := rand.Perm(len(stations))
		count := TasksPerPlayer
		if count > len(stations) {
			count = len(stations)
		}

		tasks := make([]TaskAssignment, count)
		for i := 0; i < count; i++ {
			tasks[i] = TaskAssignment{
				StationID: stations[perm[i]].ID,
				Completed: false,
			}
		}
		assignments[pid] = tasks

		// Only crewmate tasks count toward the total
		if role == RoleCrewmate {
			totalTasks += count
		}
	}

	return &TasksState{
		Stations:    stations,
		Assignments: assignments,
		TotalTasks:  totalTasks,
		DoneTasks:   0,
	}
}

// GetPlayerTasks returns a player's task list with station info
func (ts *TasksState) GetPlayerTasks(playerID string) []PlayerTaskInfo {
	assignments, ok := ts.Assignments[playerID]
	if !ok {
		return nil
	}

	stationMap := make(map[string]TaskStation)
	for _, s := range ts.Stations {
		stationMap[s.ID] = s
	}

	result := make([]PlayerTaskInfo, len(assignments))
	for i, a := range assignments {
		station := stationMap[a.StationID]
		result[i] = PlayerTaskInfo{
			StationID: a.StationID,
			Type:      station.Type,
			RoomID:    station.RoomID,
			Position:  station.Position,
			Completed: a.Completed,
		}
	}
	return result
}

// CompleteTask marks a task as done. Returns true if valid completion.
func (ts *TasksState) CompleteTask(playerID string, stationID string, role Role) bool {
	assignments, ok := ts.Assignments[playerID]
	if !ok {
		return false
	}

	for i, a := range assignments {
		if a.StationID == stationID && !a.Completed {
			assignments[i].Completed = true
			ts.Assignments[playerID] = assignments

			// Only crewmate completions count
			if role == RoleCrewmate {
				ts.DoneTasks++
			}
			return true
		}
	}
	return false
}

// AllTasksDone returns true if crewmates have completed all required tasks
func (ts *TasksState) AllTasksDone() bool {
	return ts.DoneTasks >= ts.TotalTasks
}

// Progress returns a value between 0.0 and 1.0
func (ts *TasksState) Progress() float64 {
	if ts.TotalTasks == 0 {
		return 1.0
	}
	return float64(ts.DoneTasks) / float64(ts.TotalTasks)
}
