package main

import (
	"math/rand"
	"strings"
	"time"
)

// Perception: bots only learn things a human in their spot could know.
// Crewmate bots build a suspicion score per player from:
//
//   - seeing someone tag            -> 1.0 (certain, and remembered)
//   - last saw someone with the victim -> up to +0.45 (fades over 30s)
//   - seeing someone near a body    -> +0.35 (could be the tagger, could be
//     an innocent who wandered by)
//   - catching a false location claim ("I was in Engine" but the bot was
//     looking right at them in Reactor) -> +0.3; a claim that matches -> -0.1
//   - hearing "X is sus" in chat    -> +0.15 per accuser (ignored if the
//     accuser is someone the bot already suspects)
//   - being with someone just before a meeting -> -0.15 (alibi)
//
// Suspicion fades after each meeting (how much depends on difficulty),
// except for witnessed tags.

const (
	aiSusWitnessed  = 1.0
	aiSusNearBody   = 0.35
	aiSusChat       = 0.15
	aiSusAlibi      = 0.15
	aiSusSeenWith   = 0.45 // last saw them with the victim (fades with time)
	aiSusLying      = 0.3  // location claim contradicts what the bot saw
	aiSusConfirmed  = 0.1  // location claim matches what the bot saw
	aiSeenWithRange = 200.0
	aiSeenWithFor   = 30 * time.Second
	aiClaimWindow   = 3 * time.Second // only "you were right next to me" sightings can expose a lie
	aiAlibiWindow   = 10 * time.Second
	aiSusVoteSure   = 0.5  // vote for them
	aiSusVoteMaybe  = 0.35 // sometimes vote for them, usually skip
	aiIgnoreAccuser = 0.5  // don't listen to accusations from players this sus
)

// aiVisionRadius mirrors the client's fog of war: crewmate bots see less
// during Lights Out, just like human crewmates.
func aiVisionRadius(gs *GameState, ai *AIBrain) float64 {
	if ai.Role == RoleCrewmate && gs.Sabotage.Active == SabotageLightsOut {
		return AIReducedVision
	}
	return AIVisionRadius
}

// aiPerceive updates what the bot knows from what it can currently see.
// Called every decision tick while gs.mu is held.
func aiPerceive(gs *GameState, ai *AIBrain, pos Vec2, now time.Time) {
	vision := aiVisionRadius(gs, ai)
	var visible []string
	for otherID, otherPos := range gs.Positions {
		if otherID == ai.PlayerID || gs.Frozen[otherID] {
			continue
		}
		if withinRange(pos, otherPos, vision) {
			visible = append(visible, otherID)
			ai.NearbyLog[otherID] = now
			if room := gs.Map.GetRoomAt(otherPos); room != "" {
				ai.SeenInRoom[otherID] = room
			}
		}
	}

	if ai.Role != RoleCrewmate {
		return
	}

	// Remember who was hanging around together — if one of them turns up
	// frozen later, the other has some explaining to do.
	for i, a := range visible {
		for _, b := range visible[i+1:] {
			if withinRange(gs.Positions[a], gs.Positions[b], aiSeenWithRange) {
				aiRecordSeenWith(ai, a, b, now)
				aiRecordSeenWith(ai, b, a, now)
			}
		}
	}

	notice := aiProfiles[ai.Difficulty].NoticeChance
	for bodyID, bodyPos := range gs.BodyPos {
		if gs.BodyReported[bodyID] || !withinRange(pos, bodyPos, vision) {
			continue
		}
		ai.KnownBodies[bodyID] = true

		// Who's hanging around the body where I can see them?
		seen := ai.SeenAtBody[bodyID]
		if seen == nil {
			seen = make(map[string]bool)
			ai.SeenAtBody[bodyID] = seen
		}
		for otherID, otherPos := range gs.Positions {
			if otherID == ai.PlayerID || gs.Frozen[otherID] || seen[otherID] {
				continue
			}
			if !withinRange(pos, otherPos, vision) || !withinRange(bodyPos, otherPos, AINearBodyRange) {
				continue
			}
			seen[otherID] = true
			if rand.Float64() < notice {
				ai.Suspicion[otherID] += aiSusNearBody
			}
		}
	}
}

func aiRecordSeenWith(ai *AIBrain, a, b string, now time.Time) {
	m := ai.SeenWith[a]
	if m == nil {
		m = make(map[string]time.Time)
		ai.SeenWith[a] = m
	}
	m[b] = now
}

// aiNotifyTag tells every crewmate bot that can see the tagger that a tag
// just happened. Called from tagLocked, so it covers human and AI taggers.
// Caller must hold gs.mu.
func aiNotifyTag(gs *GameState, taggerID, victimID string, now time.Time) {
	taggerPos := gs.Positions[taggerID]
	for id, ai := range gs.AIBrains {
		// A tagger bot that sees its partner tag knows their cooldown restarted
		if ai.isPartner(taggerID) && withinRange(gs.Positions[id], taggerPos, aiVisionRadius(gs, ai)) {
			ai.PartnerTagSeen[taggerID] = now
			continue
		}
		if ai.Role != RoleCrewmate || id == victimID || gs.Frozen[id] {
			continue
		}
		if !withinRange(gs.Positions[id], taggerPos, aiVisionRadius(gs, ai)) {
			continue
		}
		if rand.Float64() >= aiProfiles[ai.Difficulty].NoticeChance {
			continue
		}
		dbg("AI %s witnessed %s tag %s", id, taggerID, victimID)
		ai.Witnessed[taggerID] = true
		ai.Suspicion[taggerID] = aiSusWitnessed
		ai.KnownBodies[victimID] = true
	}
}

// aiNotifyChat lets bots hear a meeting message. Accusations ("sus_<color>")
// and location claims ("loc_<room>") carry information. Caller must hold gs.mu.
func aiNotifyChat(gs *GameState, senderID, messageID string, now time.Time) {
	if room, ok := strings.CutPrefix(messageID, "loc_"); ok {
		aiHearLocationClaim(gs, senderID, room, now)
		return
	}
	color, ok := strings.CutPrefix(messageID, "sus_")
	if !ok {
		return
	}
	targetID := ""
	for id, c := range gs.PlayerColors {
		if c == color {
			targetID = id
			break
		}
	}
	if targetID == "" || targetID == senderID {
		return
	}

	for id, ai := range gs.AIBrains {
		if id == senderID {
			continue
		}
		key := senderID + ">" + targetID
		if ai.HeardFrom[key] {
			continue // repeating yourself doesn't make it more true
		}
		ai.HeardFrom[key] = true

		if ai.Accusers[targetID] == nil {
			ai.Accusers[targetID] = make(map[string]bool)
		}
		ai.Accusers[targetID][senderID] = true

		if targetID == id {
			ai.AccusedMe++
			continue
		}
		ai.Accusations[targetID]++

		if ai.Role == RoleCrewmate && ai.Suspicion[senderID] < aiIgnoreAccuser {
			ai.Suspicion[targetID] += aiSusChat
		}
	}
}

// aiHearLocationClaim checks "I was in <room>" against what each crewmate
// bot actually saw. Only recent sightings count — people move around.
func aiHearLocationClaim(gs *GameState, senderID, room string, now time.Time) {
	// Judge against sightings from just before the meeting, not from when
	// the message arrives — discussion can run for a minute.
	ref := now
	if gs.Meeting != nil {
		ref = gs.Meeting.StartTime
	}
	for id, ai := range gs.AIBrains {
		if id == senderID || ai.Role != RoleCrewmate {
			continue
		}
		key := senderID + "@loc"
		if ai.HeardFrom[key] {
			continue // only judge someone's first location claim
		}
		seenAt, seen := ai.NearbyLog[senderID]
		seenRoom := ai.SeenInRoom[senderID]
		if !seen || seenRoom == "" || ref.Sub(seenAt) > aiClaimWindow {
			continue
		}
		ai.HeardFrom[key] = true
		if seenRoom == room {
			ai.Suspicion[senderID] -= aiSusConfirmed
			if ai.Suspicion[senderID] < 0 {
				ai.Suspicion[senderID] = 0
			}
		} else {
			dbg("AI %s caught %s (%s) lying: claimed %s, seen in %s", id, senderID, gs.Roles[senderID], room, seenRoom)
			ai.Suspicion[senderID] += aiSusLying
		}
	}
}

func aiOnMeetingStart(gs *GameState, ai *AIBrain, now time.Time) {
	ai.InMeeting = true
	ai.MeetingVoted = false
	ai.MeetingChats = 0
	ai.LastChat = ""
	ai.VoteDelay = time.Duration(2+rand.Intn(8)) * time.Second
	ai.Accusations = make(map[string]int)
	ai.Accusers = make(map[string]map[string]bool)
	ai.HeardFrom = make(map[string]bool)
	ai.AccusedMe = 0
	ai.SaidISaw = false
	ai.SaidNotMe = false
	ai.SaidDefend = false
	ai.BodyRoom = ""
	ai.Goal = GoalIdle
	ai.GoalTarget = ""
	ai.Path = nil

	victim := gs.Meeting.BodyID
	if pos, ok := gs.BodyPos[victim]; ok {
		ai.BodyRoom = gs.Map.GetRoomAt(pos)
	}

	if ai.Role != RoleCrewmate {
		return
	}

	// Who did I last see with the victim? Recent sightings count most.
	for otherID, seenAt := range ai.SeenWith[victim] {
		age := now.Sub(seenAt)
		if otherID == ai.PlayerID || gs.Frozen[otherID] || age > aiSeenWithFor {
			continue
		}
		fade := 1 - age.Seconds()/aiSeenWithFor.Seconds()
		ai.Suspicion[otherID] += aiSusSeenWith * fade
		dbg("AI %s last saw %s with victim %s %.0fs ago", ai.PlayerID, otherID, victim, age.Seconds())
	}

	// Alibi: someone who was right next to me a moment ago probably wasn't
	// off freezing people.
	{
		for id, seenAt := range ai.NearbyLog {
			if ai.Witnessed[id] || now.Sub(seenAt) > aiAlibiWindow {
				continue
			}
			ai.Suspicion[id] -= aiSusAlibi
			if ai.Suspicion[id] < 0 {
				ai.Suspicion[id] = 0
			}
		}
	}
}

func aiOnMeetingEnd(gs *GameState, ai *AIBrain, now time.Time) {
	ai.InMeeting = false
	ai.RoundStart = now
	for id := range ai.Suspicion {
		if gs.Frozen[id] {
			delete(ai.Suspicion, id)
		} else if !ai.Witnessed[id] {
			ai.Suspicion[id] *= aiProfiles[ai.Difficulty].SusDecay
		}
	}
	ai.KnownBodies = make(map[string]bool)
	ai.SeenAtBody = make(map[string]map[string]bool)

	// Everyone was teleported to the cafeteria — old paths are useless.
	ai.Goal = GoalIdle
	ai.GoalTarget = ""
	ai.Path = nil
	ai.PathIdx = 0
	ai.StuckTicks = 0
	ai.LastPos = gs.Positions[ai.PlayerID]
	ai.NextDecision = now
}

// topSuspect returns the most suspicious player among candidates.
func (ai *AIBrain) topSuspect(candidates []string) (string, float64) {
	best, bestSus := "", 0.0
	for _, id := range candidates {
		if id == ai.PlayerID {
			continue
		}
		if s := ai.Suspicion[id]; s > bestSus {
			best, bestSus = id, s
		}
	}
	return best, bestSus
}

// mostAccused returns the candidate others have accused most this meeting.
func (ai *AIBrain) mostAccused(candidates []string) (string, int) {
	best, bestN := "", 0
	for _, id := range candidates {
		if id == ai.PlayerID {
			continue
		}
		if n := ai.Accusations[id]; n > bestN {
			best, bestN = id, n
		}
	}
	return best, bestN
}

func (ai *AIBrain) isPartner(id string) bool {
	for _, p := range ai.Partners {
		if p == id {
			return true
		}
	}
	return false
}

// withoutPartners filters a tagger's partners out of a candidate list.
func (ai *AIBrain) withoutPartners(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !ai.isPartner(id) {
			out = append(out, id)
		}
	}
	return out
}

// partnerAccuser returns an alive player who accused one of this tagger's
// partners this meeting, so the bot can turn suspicion back on them.
func (ai *AIBrain) partnerAccuser(alive []string) string {
	for _, p := range ai.Partners {
		for _, id := range alive {
			if id != ai.PlayerID && !ai.isPartner(id) && ai.Accusers[p][id] {
				return id
			}
		}
	}
	return ""
}

// partnerPick returns an alive crewmate a partner accused this meeting, so
// the bot can back them up.
func (ai *AIBrain) partnerPick(alive []string) string {
	for _, id := range alive {
		if id == ai.PlayerID || ai.isPartner(id) {
			continue
		}
		for _, p := range ai.Partners {
			if ai.Accusers[id][p] {
				return id
			}
		}
	}
	return ""
}
