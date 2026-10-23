package main

import (
	"encoding/json"
	"testing"
	"time"
)

// attachTestClients gives each listed player a Client whose outgoing
// messages can be inspected, and registers the room so handlers find it.
func attachTestClients(t *testing.T, room *Room, ids ...string) map[string]*Client {
	t.Helper()
	clients := make(map[string]*Client)
	for _, id := range ids {
		c := &Client{id: id, roomCode: room.Code, send: make(chan []byte, 256)}
		room.Clients[id] = c
		clients[id] = c
	}
	roomsMu.Lock()
	rooms[room.Code] = room
	roomsMu.Unlock()
	t.Cleanup(func() {
		roomsMu.Lock()
		delete(rooms, room.Code)
		roomsMu.Unlock()
	})
	return clients
}

// cooldownsSent drains a client's queue and returns the cooldown payloads.
func cooldownsSent(c *Client) []CooldownPayload {
	var out []CooldownPayload
	for {
		select {
		case raw := <-c.send:
			var env Envelope
			if json.Unmarshal(raw, &env) == nil && env.Type == MsgCooldown {
				var p CooldownPayload
				json.Unmarshal(env.Payload, &p)
				out = append(out, p)
			}
		default:
			return out
		}
	}
}

func TestSabotageSendsSabotageCooldown(t *testing.T) {
	roles := map[string]Role{"t": RoleTagger, "a": RoleCrewmate, "b": RoleCrewmate}
	room, _ := newTestGame(roles, nil)
	clients := attachTestClients(t, room, "t")

	payload, _ := json.Marshal(SabotagePayload{Type: SabotageLightsOut})
	clients["t"].handleSabotage(payload)

	cds := cooldownsSent(clients["t"])
	if len(cds) != 1 || cds[0].Kind != CooldownSabotage || cds[0].Seconds != SabotageCooldown.Seconds() {
		t.Fatalf("got cooldowns %+v, want one sabotage cooldown of %.0fs", cds, SabotageCooldown.Seconds())
	}
}

func TestTagSendsTagCooldown(t *testing.T) {
	roles := map[string]Role{"t": RoleTagger, "a": RoleCrewmate, "b": RoleCrewmate}
	room, gs := newTestGame(roles, nil)
	clients := attachTestClients(t, room, "t")
	gs.TagCooldowns["t"] = time.Now().Add(-time.Second)
	gs.Positions["t"] = Vec2{300, 300}
	gs.Positions["a"] = Vec2{330, 300}

	payload, _ := json.Marshal(TagPlayerPayload{TargetID: "a"})
	clients["t"].handleTagPlayer(payload)

	cds := cooldownsSent(clients["t"])
	if len(cds) != 1 || cds[0].Kind != CooldownTag || cds[0].Seconds != gs.Settings.TagCooldown {
		t.Fatalf("got cooldowns %+v, want one tag cooldown of %.0fs", cds, gs.Settings.TagCooldown)
	}
}

func TestMeetingEndResendsTagCooldown(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the 3s meeting reveal")
	}
	roles := map[string]Role{"t": RoleTagger, "a": RoleCrewmate, "b": RoleCrewmate}
	room, gs := newTestGame(roles, nil)
	clients := attachTestClients(t, room, "t", "a")

	meeting := NewMeeting("a", "emergency", "", []string{"t", "a", "b"})
	gs.Meeting = meeting
	room.Phase = PhaseVoting
	runMeetingTimeline(room, meeting, 0, 0)

	cds := cooldownsSent(clients["t"])
	if len(cds) != 1 || cds[0].Kind != CooldownTag || cds[0].Seconds != TagCooldownAfterMeeting.Seconds() {
		t.Fatalf("tagger got cooldowns %+v, want one %.0fs tag cooldown", cds, TagCooldownAfterMeeting.Seconds())
	}
	if cds := cooldownsSent(clients["a"]); len(cds) != 0 {
		t.Errorf("crewmate was sent cooldowns %+v", cds)
	}
}
