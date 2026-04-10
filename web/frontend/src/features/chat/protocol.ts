import type { ToolCall, ToolCallStatus } from "@/components/chat/tool-call-card"
import { normalizeUnixTimestamp } from "@/features/chat/state"
import { updateChatStore } from "@/store/chat"
import { updateCouncilDetail, updateCouncilStream } from "@/store/council"

export interface PicoMessage {
  type: string
  id?: string
  session_id?: string
  timestamp?: number | string
  payload?: Record<string, unknown>
}

export function handlePicoMessage(
  message: PicoMessage,
  expectedSessionId: string,
) {
  if (message.session_id && message.session_id !== expectedSessionId) {
    return
  }

  const payload = message.payload || {}

  switch (message.type) {
    case "message.create": {
      const content = (payload.content as string) || ""
      const messageId = (payload.message_id as string) || `pico-${Date.now()}`
      const timestamp =
        message.timestamp !== undefined &&
        Number.isFinite(Number(message.timestamp))
          ? normalizeUnixTimestamp(Number(message.timestamp))
          : Date.now()

      updateChatStore((prev) => ({
        messages: [
          ...prev.messages,
          {
            id: messageId,
            role: "assistant",
            content,
            timestamp,
          },
        ],
        isTyping: false,
        typingStatus: "thinking",
      }))
      break
    }

    case "message.update": {
      const content = (payload.content as string) || ""
      const messageId = payload.message_id as string
      if (!messageId) {
        break
      }

      updateChatStore((prev) => ({
        messages: prev.messages.map((msg) =>
          msg.id === messageId ? { ...msg, content } : msg,
        ),
      }))
      break
    }

    case "typing.start":
      updateChatStore({ isTyping: true, typingStatus: "thinking" })
      break

    case "typing.stop":
      updateChatStore({ isTyping: false, typingStatus: "thinking" })
      break

    // tool.exec.start — sent when a tool begins executing
    case "tool.exec.start": {
      const toolName = (payload.tool as string) || "unknown"
      const toolId = (payload.tool_id as string) || `tool-${Date.now()}`
      const inputRaw = payload.input as Record<string, unknown> | undefined

      const toolCall: ToolCall = {
        id: toolId,
        tool: toolName,
        status: "running",
        input: inputRaw,
      }

      // Attach to the last assistant message, or create a new one if the last
      // message is from the user (tool feedback before a reply).
      updateChatStore((prev) => {
        const messages = [...prev.messages]
        const lastIdx = messages.length - 1
        const lastMsg = messages[lastIdx]

        if (lastMsg && lastMsg.role === "assistant") {
          messages[lastIdx] = {
            ...lastMsg,
            toolCalls: [...(lastMsg.toolCalls ?? []), toolCall],
          }
          return { messages, typingStatus: "tool" }
        }

        // No assistant message yet — create a tool-only stub
        messages.push({
          id: `tool-stub-${Date.now()}`,
          role: "assistant",
          content: "",
          timestamp: Date.now(),
          toolCalls: [toolCall],
        })
        return { messages, typingStatus: "tool" }
      })
      break
    }

    // tool.exec.end — sent when a tool finishes
    case "tool.exec.end": {
      const toolId = payload.tool_id as string
      if (!toolId) {
        break
      }

      const status: ToolCallStatus = (payload.is_error as boolean)
        ? "error"
        : "completed"
      const output = payload.output as string | undefined
      const errorMessage =
        status === "error" ? (payload.error as string | undefined) : undefined

      updateChatStore((prev) => ({
        messages: prev.messages.map((msg) => {
          if (!msg.toolCalls) return msg
          const updated = msg.toolCalls.map((tc) =>
            tc.id === toolId
              ? { ...tc, status, output, errorMessage }
              : tc,
          )
          return { ...msg, toolCalls: updated }
        }),
        typingStatus: "thinking",
      }))
      break
    }

    // subturn.spawn — emitted when a sub-agent is spawned
    case "subturn.spawn":
      updateChatStore({ typingStatus: "spawn" })
      break

    // subturn.end — sub-agent finished
    case "subturn.end":
      updateChatStore({ typingStatus: "thinking" })
      break

    case "error":
      console.error("Pico error:", payload)
      updateChatStore({ isTyping: false, typingStatus: "thinking" })
      break

    case "pong":
      break

    case "council.start": {
      const id = payload.council_id as string
      const title = payload.title as string
      const description = (payload.description as string) || ""
      const roster = (payload.roster as string[]) || []
      updateCouncilDetail((prev) => {
        if (prev && prev.id === id) return prev
        return {
          id,
          title,
          description,
          roster,
          status: "active",
          rounds: 0,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          transcript: [],
        }
      })
      updateCouncilStream({
        activeCouncilId: id,
        speakingAgentId: null,
        speakingAgentType: null,
        streamingContent: "",
        currentRound: 0,
      })
      break
    }

    case "council.agent.start": {
      const agentId = payload.agent_id as string
      const agentType = (payload.agent_type as string) || ""
      const round = payload.round as number
      updateCouncilStream({
        speakingAgentId: agentId,
        speakingAgentType: agentType,
        streamingContent: "",
        currentRound: round,
      })
      break
    }

    case "council.agent.delta": {
      const delta = payload.delta as string
      updateCouncilStream((prev) => ({
        streamingContent: prev.streamingContent + delta,
      }))
      break
    }

    case "council.agent.end": {
      const councilId = payload.council_id as string
      const agentId = payload.agent_id as string
      const agentType = (payload.agent_type as string) || ""
      const content = payload.content as string
      const round = payload.round as number
      updateCouncilDetail((prev) => {
        if (!prev || prev.id !== councilId) return prev
        return {
          ...prev,
          transcript: [
            ...prev.transcript,
            {
              role: "agent",
              agent_id: agentId,
              agent_type: agentType,
              content,
              round,
              ts: new Date().toISOString(),
            },
          ],
        }
      })
      updateCouncilStream({
        speakingAgentId: null,
        speakingAgentType: null,
        streamingContent: "",
      })
      break
    }

    case "council.round.end": {
      const councilId = payload.council_id as string
      const round = payload.round as number
      const decision = (payload.decision as string) || ""
      if (decision && decision.toLowerCase() !== "conclude") {
        updateCouncilDetail((prev) => {
          if (!prev || prev.id !== councilId) return prev
          return {
            ...prev,
            rounds: round,
            transcript: [
              ...prev.transcript,
              {
                role: "moderator",
                content: decision,
                round,
                ts: new Date().toISOString(),
              },
            ],
          }
        })
      }
      break
    }

    case "council.end": {
      const councilId = payload.council_id as string
      const synthesis = (payload.synthesis as string) || ""
      const rounds = (payload.rounds as number) || 0
      updateCouncilDetail((prev) => {
        if (!prev || prev.id !== councilId) return prev
        const transcript = [...prev.transcript]
        if (synthesis) {
          transcript.push({
            role: "synthesis",
            content: synthesis,
            round: rounds,
            ts: new Date().toISOString(),
          })
        }
        return { ...prev, status: "closed", rounds, transcript }
      })
      updateCouncilStream({
        activeCouncilId: null,
        speakingAgentId: null,
        speakingAgentType: null,
        streamingContent: "",
        currentRound: 0,
      })
      break
    }

    default:
      console.log("Unknown pico message type:", message.type)
  }
}
