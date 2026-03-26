import { IconCheck, IconCopy } from "@tabler/icons-react"
import { useState } from "react"

import { MarkdownRenderer } from "@/components/chat/markdown-renderer"
import { ToolCallCard } from "@/components/chat/tool-call-card"
import type { ToolCall } from "@/components/chat/tool-call-card"
import { Button } from "@/components/ui/button"
import { formatMessageTime } from "@/hooks/use-pico-chat"

interface AssistantMessageProps {
  content: string
  timestamp?: string | number
  toolCalls?: ToolCall[]
}

export function AssistantMessage({
  content,
  timestamp = "",
  toolCalls,
}: AssistantMessageProps) {
  const [isCopied, setIsCopied] = useState(false)
  const formattedTimestamp =
    timestamp !== "" ? formatMessageTime(timestamp) : ""

  const handleCopy = () => {
    navigator.clipboard.writeText(content).then(() => {
      setIsCopied(true)
      setTimeout(() => setIsCopied(false), 2000)
    })
  }

  const hasToolCalls = toolCalls && toolCalls.length > 0
  const hasContent = content.trim().length > 0

  return (
    <div className="group flex w-full flex-col gap-1.5">
      <div className="text-muted-foreground flex items-center justify-between gap-2 px-1 text-xs opacity-70">
        <div className="flex items-center gap-2">
          <span>Spawnbot</span>
          {formattedTimestamp && (
            <>
              <span className="opacity-50">•</span>
              <span>{formattedTimestamp}</span>
            </>
          )}
        </div>
      </div>

      {hasToolCalls && (
        <div className="flex flex-col gap-1.5 py-1">
          {toolCalls.map((tc) => (
            <ToolCallCard key={tc.id} toolCall={tc} />
          ))}
        </div>
      )}

      {hasContent && (
        <div className="bg-card text-card-foreground relative overflow-hidden rounded-xl border">
          <MarkdownRenderer content={content} className="p-4" />
          <Button
            variant="ghost"
            size="icon"
            className="bg-background/50 hover:bg-background/80 absolute top-2 right-2 h-7 w-7 opacity-0 transition-opacity group-hover:opacity-100"
            onClick={handleCopy}
          >
            {isCopied ? (
              <IconCheck className="h-4 w-4 text-green-500" />
            ) : (
              <IconCopy className="text-muted-foreground h-4 w-4" />
            )}
          </Button>
        </div>
      )}

      {!hasContent && !hasToolCalls && (
        <div className="bg-card text-card-foreground relative overflow-hidden rounded-xl border">
          <MarkdownRenderer content={content} className="p-4" />
        </div>
      )}
    </div>
  )
}
