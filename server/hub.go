package main

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.handleDisconnect(client)
			}
		}
	}
}

func (h *Hub) handleDisconnect(client *Client) {
	if client.roomCode == "" {
		return
	}

	roomsMu.RLock()
	room, exists := rooms[client.roomCode]
	roomsMu.RUnlock()

	if !exists {
		return
	}

	room.removePlayer(client.id)

	// Check if room still exists (it may have been cleaned up if empty)
	roomsMu.RLock()
	_, stillExists := rooms[client.roomCode]
	roomsMu.RUnlock()

	if !stillExists {
		return
	}

	// Notify remaining players of the new room state
	ids := room.snapshotPlayerIDs()
	for _, pid := range ids {
		room.sendTo(pid, MsgRoomState, room.roomStatePayload(pid))
	}

	room.mu.RLock()
	inGame := room.Game != nil && room.Phase == PhasePlaying
	room.mu.RUnlock()

	// If a game is in progress, check win conditions and task progress
	if inGame {
		// Re-broadcast task progress to all (since total may have changed)
		for _, id := range ids {
			tasks := room.Game.Tasks.GetPlayerTasks(id)
			room.sendTo(id, MsgTaskProgress, TaskProgressPayload{
				Progress: room.Game.Tasks.Progress(),
				Tasks:    tasks,
			})
		}

		checkWinConditions(room)
	}
}
