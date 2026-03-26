import type { ToolCall, ToolCallStatus } from "@/components/chat/tool-call-card"
import { normalizeUnixTimestamp } from "@/features/chat/state"
import { updateChatStore } from "@/store/chat"

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

    default:
      console.log("Unknown pico message type:", message.type)
  }
}
