"use client";

import { useEffect, useRef, useState } from "react";
import { useGameStore } from "@/stores/gameStore";
import { sounds } from "@/lib/sounds";
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

  // Match server's MaxChatMessages — bumped from 3 to 8
  const MAX_MESSAGES = 8;
  const myMessageCount = chatMessages.filter((m) => m.senderId === myId).length;
  const myMessagesLeft = Math.max(0, MAX_MESSAGES - myMessageCount);

  const handleSendMessage = (msgId: string) => {
    sounds.click();
    send("chat_message", { messageId: msgId });
  };

  const handleVote = (targetId: string) => {
    if (!isAlive || meetingPhase !== "voting") return;
    if (myVote !== null) return; // already voted
    sounds.vote();
    setMyVote(targetId);
    send("cast_vote", { targetId });
  };

  // Group chat messages by sender (used for tiny bubbles above portraits)
  const messagesByPlayer: Record<string, string[]> = {};
  for (const msg of chatMessages) {
    if (!messagesByPlayer[msg.senderId]) {
      messagesByPlayer[msg.senderId] = [];
    }
    messagesByPlayer[msg.senderId].push(msg.messageId);
  }

  // Auto-scroll chat log to the bottom when new messages arrive
  const chatLogRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (chatLogRef.current) {
      chatLogRef.current.scrollTop = chatLogRef.current.scrollHeight;
    }
  }, [chatMessages.length]);

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
    <div className="absolute inset-0 z-30 flex flex-col bg-black/85 backdrop-blur p-2 sm:p-4 overflow-y-auto">
      {/* Header */}
      <div className="text-center mb-3 sm:mb-4">
        <h1 className="text-xl sm:text-2xl font-black text-white">
          {meeting.reason === "body" ? "🚨 Frozen Friend Found!" : "🚨 Emergency Meeting!"}
        </h1>
        <p className="text-gray-400 text-xs sm:text-sm">Called by {callerName}</p>
        <div className="mt-1.5 inline-block px-3 sm:px-4 py-1 rounded-full bg-gray-800">
          <span className="text-white font-bold text-sm sm:text-base">
            {meetingPhase === "discussion" ? "Discuss: " : "Vote: "}
            {remaining}s
          </span>
        </div>
      </div>

      {/* Players grid for voting */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 sm:gap-3 max-w-2xl mx-auto w-full mb-3 sm:mb-4">
        {alivePlayers.map((player) => {
          const messages = messagesByPlayer[player.id] || [];
          const isVoted = myVote === player.id;
          return (
            <button
              key={player.id}
              onClick={() => handleVote(player.id)}
              disabled={meetingPhase !== "voting" || !isAlive || myVote !== null}
              className={`relative flex flex-col items-center gap-1.5 sm:gap-2 p-2 sm:p-3 rounded-xl transition-all ${
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
                className="w-12 h-12 sm:w-14 sm:h-14 rounded-full flex items-center justify-center text-lg sm:text-xl font-bold border-2 border-black"
                style={{ backgroundColor: COLOR_HEX[player.color] }}
              >
                {player.name[0]?.toUpperCase()}
              </div>
              <span className="text-white text-xs sm:text-sm font-medium truncate max-w-full">
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
            className={`flex flex-col items-center gap-1.5 sm:gap-2 p-2 sm:p-3 rounded-xl transition-all ${
              myVote === ""
                ? "bg-blue-600 ring-4 ring-blue-300"
                : myVote === null
                ? "bg-gray-800 hover:bg-gray-700"
                : "bg-gray-800/60"
            }`}
          >
            <div className="w-12 h-12 sm:w-14 sm:h-14 rounded-full bg-gray-700 flex items-center justify-center text-2xl">
              ⏭️
            </div>
            <span className="text-white text-xs sm:text-sm font-medium">
              Skip
            </span>
          </button>
        )}
      </div>

      {/* Chat log — what everyone is saying */}
      <div className="max-w-2xl mx-auto w-full mb-3">
        <p className="text-gray-400 text-xs mb-1 text-center">Chat</p>
        <div
          ref={chatLogRef}
          className="bg-gray-900/70 rounded-xl p-2 sm:p-3 max-h-24 sm:max-h-32 overflow-y-auto"
        >
          {chatMessages.length === 0 ? (
            <p className="text-gray-600 text-xs sm:text-sm text-center italic py-1">
              No one has said anything yet...
            </p>
          ) : (
            <ul className="flex flex-col gap-1.5">
              {chatMessages.map((msg, i) => {
                const sender = players.find((p) => p.id === msg.senderId);
                const m = QUICK_MESSAGES.find((q) => q.id === msg.messageId);
                if (!sender || !m) return null;
                return (
                  <li
                    key={i}
                    className="flex items-center gap-1.5 sm:gap-2 text-xs sm:text-sm min-w-0"
                  >
                    <div
                      className="w-5 h-5 sm:w-6 sm:h-6 rounded-full flex items-center justify-center text-[10px] sm:text-xs font-bold border border-black shrink-0"
                      style={{ backgroundColor: COLOR_HEX[sender.color] }}
                    >
                      {sender.name[0]?.toUpperCase()}
                    </div>
                    <span className="text-white font-medium shrink-0">
                      {sender.name}:
                    </span>
                    <span className="text-base sm:text-xl shrink-0">
                      {m.icon}
                    </span>
                    <span className="text-gray-200 truncate">{m.text}</span>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>

      {/* Quick chat messages */}
      {isAlive && (
        <div className="max-w-2xl mx-auto w-full mt-2">
          <div className="flex items-center justify-center gap-2 mb-2">
            <p className="text-gray-400 text-xs">Quick messages</p>
            <span
              className={`text-xs font-semibold ${
                myMessagesLeft === 0 ? "text-red-400" : "text-gray-500"
              }`}
            >
              ({myMessagesLeft} left)
            </span>
          </div>
          <div className="grid grid-cols-4 sm:grid-cols-7 gap-2">
            {QUICK_MESSAGES.map((msg) => (
              <button
                key={msg.id}
                onClick={() => handleSendMessage(msg.id)}
                disabled={myMessagesLeft === 0}
                className="flex flex-col items-center gap-1 p-2 rounded-lg bg-gray-800 hover:bg-gray-700 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
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
