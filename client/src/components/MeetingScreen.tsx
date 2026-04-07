"use client";

import { useEffect, useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { COLOR_HEX, QUICK_MESSAGES, type MsgType } from "@/lib/protocol";

interface Props {
  send: (type: MsgType, payload?: unknown) => void;
}

export default function MeetingScreen({ send }: Props) {
  const {
    meeting,
    meetingPhase,
    meetingPhaseEnd,
    chatMessages,
    myVote,
    myId,
    players,
    setMyVote,
    meetingResult,
  } = useGameStore();

  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(id);
  }, []);

  // Auto-transition from discussion → voting when discussion timer expires
  useEffect(() => {
    if (
      meetingPhase === "discussion" &&
      meeting &&
      Date.now() >= meetingPhaseEnd
    ) {
      useGameStore.setState({
        meetingPhase: "voting",
        meetingPhaseEnd: Date.now() + meeting.votingTime * 1000,
      });
    }
  }, [now, meetingPhase, meetingPhaseEnd, meeting]);

  if (!meeting) return null;

  const remaining = Math.max(0, Math.ceil((meetingPhaseEnd - now) / 1000));
  const alivePlayers = players.filter((p) => meeting.alivePlayers.includes(p.id));
  const isAlive = myId ? meeting.alivePlayers.includes(myId) : false;
  const callerName = players.find((p) => p.id === meeting.callerId)?.name ?? "?";

  const handleSendMessage = (msgId: string) => {
    send("chat_message", { messageId: msgId });
  };

  const handleVote = (targetId: string) => {
    if (!isAlive || meetingPhase !== "voting") return;
    if (myVote !== null) return; // already voted
    setMyVote(targetId);
    send("cast_vote", { targetId });
  };

  // Group chat messages by sender
  const messagesByPlayer: Record<string, string[]> = {};
  for (const msg of chatMessages) {
    if (!messagesByPlayer[msg.senderId]) {
      messagesByPlayer[msg.senderId] = [];
    }
    messagesByPlayer[msg.senderId].push(msg.messageId);
  }

  // Result phase
  if (meetingResult) {
    const ejectedName =
      players.find((p) => p.id === meetingResult.ejectedId)?.name ?? null;
    return (
      <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/85 backdrop-blur">
        <div className="bg-gray-900 rounded-2xl p-8 max-w-md text-center">
          <h2 className="text-3xl font-black mb-4 text-white">
            {ejectedName ? `${ejectedName} was sent home!` : "No one was sent home"}
          </h2>
          {ejectedName && (
            <p
              className={`text-xl font-bold ${
                meetingResult.wasTagger ? "text-green-400" : "text-yellow-400"
              }`}
            >
              {meetingResult.wasTagger
                ? "🎉 They were the Tagger!"
                : "😬 They were not the Tagger..."}
            </p>
          )}
          {!ejectedName && (
            <p className="text-gray-400">Tied or no votes</p>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="absolute inset-0 z-30 flex flex-col bg-black/85 backdrop-blur p-4 overflow-y-auto">
      {/* Header */}
      <div className="text-center mb-4">
        <h1 className="text-2xl font-black text-white">
          {meeting.reason === "body" ? "🚨 Frozen Friend Found!" : "🚨 Emergency Meeting!"}
        </h1>
        <p className="text-gray-400 text-sm">Called by {callerName}</p>
        <div className="mt-2 inline-block px-4 py-1 rounded-full bg-gray-800">
          <span className="text-white font-bold">
            {meetingPhase === "discussion" ? "Discuss: " : "Vote: "}
            {remaining}s
          </span>
        </div>
      </div>

      {/* Players grid for voting */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 max-w-2xl mx-auto w-full mb-4">
        {alivePlayers.map((player) => {
          const messages = messagesByPlayer[player.id] || [];
          const isVoted = myVote === player.id;
          return (
            <button
              key={player.id}
              onClick={() => handleVote(player.id)}
              disabled={meetingPhase !== "voting" || !isAlive || myVote !== null}
              className={`relative flex flex-col items-center gap-2 p-3 rounded-xl transition-all ${
                isVoted
                  ? "bg-blue-600 ring-4 ring-blue-300"
                  : meetingPhase === "voting" && isAlive && myVote === null
                  ? "bg-gray-800 hover:bg-gray-700 cursor-pointer"
                  : "bg-gray-800/60 cursor-default"
              }`}
            >
              {/* Speech bubble messages */}
              {messages.length > 0 && (
                <div className="absolute -top-2 -right-2 flex gap-1">
                  {messages.slice(-2).map((msgId, i) => {
                    const m = QUICK_MESSAGES.find((q) => q.id === msgId);
                    return (
                      <span
                        key={i}
                        className="text-xl bg-white rounded-full w-8 h-8 flex items-center justify-center shadow-lg"
                      >
                        {m?.icon}
                      </span>
                    );
                  })}
                </div>
              )}
              <div
                className="w-14 h-14 rounded-full flex items-center justify-center text-xl font-bold border-2 border-black"
                style={{ backgroundColor: COLOR_HEX[player.color] }}
              >
                {player.name[0]?.toUpperCase()}
              </div>
              <span className="text-white text-sm font-medium truncate max-w-full">
                {player.name}
                {player.id === myId && " (you)"}
              </span>
            </button>
          );
        })}

        {/* Skip vote button */}
        {meetingPhase === "voting" && isAlive && (
          <button
            onClick={() => handleVote("")}
            disabled={myVote !== null}
            className={`flex flex-col items-center gap-2 p-3 rounded-xl transition-all ${
              myVote === ""
                ? "bg-blue-600 ring-4 ring-blue-300"
                : myVote === null
                ? "bg-gray-800 hover:bg-gray-700"
                : "bg-gray-800/60"
            }`}
          >
            <div className="w-14 h-14 rounded-full bg-gray-700 flex items-center justify-center text-2xl">
              ⏭️
            </div>
            <span className="text-white text-sm font-medium">Skip</span>
          </button>
        )}
      </div>

      {/* Quick chat messages */}
      {isAlive && (
        <div className="max-w-2xl mx-auto w-full mt-2">
          <p className="text-gray-400 text-xs mb-2 text-center">
            Quick messages
          </p>
          <div className="grid grid-cols-4 sm:grid-cols-7 gap-2">
            {QUICK_MESSAGES.map((msg) => (
              <button
                key={msg.id}
                onClick={() => handleSendMessage(msg.id)}
                className="flex flex-col items-center gap-1 p-2 rounded-lg bg-gray-800 hover:bg-gray-700 transition-colors"
                title={msg.text}
              >
                <span className="text-2xl">{msg.icon}</span>
                <span className="text-[10px] text-gray-400 text-center leading-tight">
                  {msg.text}
                </span>
              </button>
            ))}
          </div>
        </div>
      )}

      {!isAlive && (
        <div className="text-center text-gray-400 mt-4">
          <p>You are frozen — you can watch but not vote.</p>
        </div>
      )}
    </div>
  );
}
