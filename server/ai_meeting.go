package main

import (
	"log"
	"math/rand"
	"time"
)

const aiMaxChats = 3 // messages per bot per meeting

// aiMeetingTick handles AI behavior during a meeting (gs.mu is held)
func aiMeetingTick(gs *GameState, room *Room, ai *AIBrain, now time.Time) {
	meeting := gs.Meeting
	phase := meeting.GetPhase()

	// Send chat messages during discussion, staggered and a bit random
	if phase == "discussion" && ai.MeetingChats < aiMaxChats {
		elapsed := now.Sub(meeting.StartTime)
		if elapsed > 3*time.Second && rand.Float64() < 0.03 { // ~3% chance per tick
			ai.MeetingChats++
			msgID := pickAIChatMessage(gs, ai)
			if msgID != "" && msgID != ai.LastChat && meeting.CanSendChat(ai.PlayerID) {
				ai.LastChat = msgID
				meeting.RecordChat(ai.PlayerID)
				aiNotifyChat(gs, ai.PlayerID, msgID, now)
				go func(r *Room, sender, msg string) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("ai chat broadcast panic: %v", rec)
						}
					}()
					r.broadcast(MsgChatMessage, ChatMessagePayload{
						SenderID:  sender,
						MessageID: msg,
					})
				}(room, ai.PlayerID, msgID)
			}
		}
	}

	// Vote during voting phase, after a per-bot random delay
	if phase == "voting" && !ai.MeetingVoted {
		discDur := time.Duration(gs.Settings.DiscussionTime * float64(time.Second))
		elapsed := now.Sub(meeting.StartTime) - discDur
		if elapsed > ai.VoteDelay {
			targetID := pickAIVoteTarget(gs, ai)
			if meeting.CastVote(ai.PlayerID, targetID) {
				ai.MeetingVoted = true
				go func(r *Room, voter string) {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("ai vote broadcast panic: %v", rec)
						}
					}()
					r.broadcast(MsgVoteCast, VoteCastPayload{VoterID: voter})
				}(room, ai.PlayerID)
			}
		}
	}
}

func susMessage(gs *GameState, playerID string) string {
	if color, ok := gs.PlayerColors[playerID]; ok {
		return "sus_" + color
	}
	return ""
}

// pickAIChatMessage returns a quick message based only on what the bot knows
func pickAIChatMessage(gs *GameState, ai *AIBrain) string {
	alive := gs.Meeting.AlivePlayers

	if ai.Role == RoleCrewmate {
		suspect, level := ai.topSuspect(alive)
		reporter := gs.Meeting.CallerID == ai.PlayerID && gs.Meeting.Reason == "body"
		switch {
		case reporter && ai.MeetingChats == 1 && ai.BodyRoom != "":
			return "loc_" + ai.BodyRoom // "found them in Medbay"
		case ai.AccusedMe > 0 && !ai.SaidNotMe:
			ai.SaidNotMe = true
			return "not_me"
		case suspect != "" && level >= aiSusVoteSure && !ai.SaidISaw:
			ai.SaidISaw = true
			return "i_saw"
		case suspect != "" && level >= aiSusVoteSure:
			return susMessage(gs, suspect)
		case ai.MeetingChats == 1 && ai.LastRoom != "":
			return "loc_" + ai.LastRoom
		case level < aiSusVoteMaybe:
			return []string{"idk", "where", "doing_task"}[rand.Intn(3)]
		default:
			return []string{"doing_task", "with_me", "where"}[rand.Intn(3)]
		}
	}

	// Tagger: blend in and back up any partner. Deny if accused, turn
	// suspicion on whoever accused a partner, echo a partner's accusation,
	// otherwise pile onto whoever the group already suspects.
	if ai.AccusedMe > 0 && !ai.SaidNotMe {
		ai.SaidNotMe = true
		return "not_me"
	}
	if accuser := ai.partnerAccuser(alive); accuser != "" && !ai.SaidDefend {
		ai.SaidDefend = true
		return susMessage(gs, accuser)
	}
	if pick := ai.partnerPick(alive); pick != "" {
		return susMessage(gs, pick)
	}
	if target, n := ai.mostAccused(ai.withoutPartners(alive)); n > 0 {
		return susMessage(gs, target)
	}
	if ai.MeetingChats == 1 && ai.LastRoom != "" && rand.Float64() < 0.6 {
		if ai.LastRoom == ai.BodyRoom {
			// Don't place yourself at the scene — claim somewhere else.
			// A bot that was watching can catch this.
			return "loc_" + aiOtherRoom(gs, ai.BodyRoom)
		}
		return "loc_" + ai.LastRoom
	}
	return []string{"doing_task", "trust_me", "idk"}[rand.Intn(3)]
}

// pickAIVoteTarget decides who the AI votes for ("" = skip)
func pickAIVoteTarget(gs *GameState, ai *AIBrain) string {
	var candidates []string
	for _, id := range gs.Meeting.AlivePlayers {
		if id != ai.PlayerID {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return ""
	}

	if ai.Role == RoleCrewmate {
		suspect, level := ai.topSuspect(candidates)
		switch {
		case level >= aiSusVoteSure:
			return suspect
		case level >= aiSusVoteMaybe && rand.Float64() < 0.35:
			return suspect
		default:
			return "" // no real evidence — skip rather than guess
		}
	}

	// Tagger: never vote out a partner. Vote whoever accused a partner,
	// then whoever a partner accused, then bandwagon; otherwise skip or
	// throw a vote at a random crewmate.
	candidates = ai.withoutPartners(candidates)
	if len(candidates) == 0 {
		return ""
	}
	if accuser := ai.partnerAccuser(candidates); accuser != "" {
		return accuser
	}
	if pick := ai.partnerPick(candidates); pick != "" {
		return pick
	}
	if target, n := ai.mostAccused(candidates); n > 0 {
		return target
	}
	if rand.Float64() < 0.5 {
		return ""
	}
	return candidates[rand.Intn(len(candidates))]
}

// aiOtherRoom picks a random room ID other than `not`.
func aiOtherRoom(gs *GameState, not string) string {
	var rooms []string
	for _, r := range gs.Map.Rooms {
		if r.ID != not {
			rooms = append(rooms, r.ID)
		}
	}
	return rooms[rand.Intn(len(rooms))]
}
