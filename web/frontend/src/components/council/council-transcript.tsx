import { useEffect, useRef, useState } from "react"
import { useAtomValue } from "jotai"

import { MarkdownRenderer } from "@/components/chat/markdown-renderer"
import {
  councilDetailAtom,
  councilStreamAtom,
  type CouncilDetail,
  type TranscriptEntry,
} from "@/store/council"

const agentColors: Record<string, { bg: string; text: string; border: string }> = {
  researcher: { bg: "#e3f2fd", text: "#1565c0", border: "#1565c0" },
  coder: { bg: "#f3e5f5", text: "#7b1fa2", border: "#7b1fa2" },
  planner: { bg: "#fff3e0", text: "#e65100", border: "#e65100" },
  reviewer: { bg: "#e8f5e9", text: "#2e7d32", border: "#2e7d32" },
}

const statusStyles: Record<string, { color: string; label: string }> = {
  active: { color: "bg-green-500", label: "Active" },
  paused: { color: "bg-orange-400", label: "Paused" },
  closed: { color: "bg-zinc-400", label: "Closed" },
}

function getEntryStyle(entry: TranscriptEntry): {
  borderColor: string
  label: string
  labelBg: string
  labelText: string
} {
  switch (entry.role) {
    case "agent": {
      const colors = agentColors[entry.agent_type ?? ""] ?? {
        border: "#9e9e9e",
        bg: "#f5f5f5",
        text: "#616161",
      }
      return {
        borderColor: colors.border,
        label: entry.agent_id ?? entry.agent_type ?? "Agent",
        labelBg: colors.bg,
        labelText: colors.text,
      }
    }
    case "moderator":
      return {
        borderColor: "#e65100",
        label: "Moderator",
        labelBg: "#fff3e0",
        labelText: "#e65100",
      }
    case "user":
      return {
        borderColor: "#9e9e9e",
        label: "User",
        labelBg: "#f5f5f5",
        labelText: "#616161",
      }
    case "synthesis":
      return {
        borderColor: "#1565c0",
        label: "Synthesis",
        labelBg: "#e3f2fd",
        labelText: "#1565c0",
      }
    default:
      return {
        borderColor: "#9e9e9e",
        label: entry.role,
        labelBg: "#f5f5f5",
        labelText: "#616161",
      }
  }
}

interface CouncilTranscriptProps {
  councilId: string
}

export function CouncilTranscript({ councilId }: CouncilTranscriptProps) {
  const [council, setCouncil] = useState<CouncilDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const bottomRef = useRef<HTMLDivElement>(null)

  // Live updates from Pico WebSocket events
  const liveCouncil = useAtomValue(councilDetailAtom)
  const stream = useAtomValue(councilStreamAtom)

  // Use live data when it matches this council, otherwise fall back to fetched data
  const displayCouncil = liveCouncil?.id === councilId ? liveCouncil : council

  useEffect(() => {
    fetch(`/api/councils/${councilId}`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`)
        return res.json()
      })
      .then((data: CouncilDetail) => {
        setCouncil(data)
        setLoading(false)
      })
      .catch((err) => {
        setError(err.message)
        setLoading(false)
      })
  }, [councilId])

  useEffect(() => {
    if (displayCouncil) {
      bottomRef.current?.scrollIntoView({ behavior: "smooth" })
    }
  }, [displayCouncil, displayCouncil?.transcript.length, stream.streamingContent])

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-muted-foreground animate-pulse text-sm">
          Loading council...
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-destructive text-sm">
          Failed to load council: {error}
        </div>
      </div>
    )
  }

  if (!displayCouncil) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-muted-foreground text-sm">Council not found</div>
      </div>
    )
  }

  const status = statusStyles[displayCouncil.status] ?? statusStyles.closed

  // Group transcript entries by round for separators
  let lastRound = -1

  return (
    <div className="mx-auto h-full max-w-4xl overflow-y-auto p-6">
      {/* Header */}
      <div className="mb-8">
        <div className="mb-2 flex items-center gap-3">
          <div
            className={`h-2.5 w-2.5 rounded-full ${status.color}`}
            title={status.label}
          />
          <h1 className="text-2xl font-semibold">{displayCouncil.title}</h1>
          <span className="text-muted-foreground text-sm">
            {displayCouncil.rounds} round{displayCouncil.rounds !== 1 ? "s" : ""}
          </span>
        </div>
        {displayCouncil.description && (
          <p className="text-muted-foreground mb-3 text-sm">
            {displayCouncil.description}
          </p>
        )}
        <div className="flex flex-wrap gap-1.5">
          {(displayCouncil.roster ?? []).map((agent) => {
            const colors = agentColors[agent] ?? {
              bg: "#f5f5f5",
              text: "#616161",
            }
            return (
              <span
                key={agent}
                className="rounded-full px-2 py-0.5 text-xs font-medium"
                style={{
                  backgroundColor: colors.bg,
                  color: colors.text,
                }}
              >
                {agent}
              </span>
            )
          })}
        </div>
      </div>

      {/* Transcript */}
      <div className="flex flex-col gap-4">
        {(displayCouncil.transcript ?? []).map((entry, idx) => {
          const style = getEntryStyle(entry)
          const showRoundSeparator = entry.round !== lastRound
          lastRound = entry.round

          return (
            <div key={idx}>
              {showRoundSeparator && (
                <div className="my-4 flex items-center gap-3">
                  <div className="border-border/40 flex-1 border-t border-dashed" />
                  <span className="text-muted-foreground text-xs font-medium">
                    Round {entry.round}
                  </span>
                  <div className="border-border/40 flex-1 border-t border-dashed" />
                </div>
              )}
              <div
                className={`rounded-lg border-l-4 p-4 ${
                  entry.role === "synthesis"
                    ? "bg-blue-50/50 dark:bg-blue-950/20"
                    : "bg-muted/20"
                }`}
                style={{ borderLeftColor: style.borderColor }}
              >
                <div className="mb-2 flex items-center gap-2">
                  <span
                    className="rounded-full px-2 py-0.5 text-xs font-medium"
                    style={{
                      backgroundColor: style.labelBg,
                      color: style.labelText,
                    }}
                  >
                    {style.label}
                  </span>
                  {entry.ts && !entry.ts.startsWith("0001") && (
                    <span className="text-muted-foreground text-xs">
                      {new Date(entry.ts).toLocaleTimeString()}
                    </span>
                  )}
                </div>
                <MarkdownRenderer content={entry.content} />
              </div>
            </div>
          )
        })}
      </div>

      {/* Live streaming indicator */}
      {stream.activeCouncilId === councilId && stream.speakingAgentId && (
        <div className="mt-4">
          <div
            className="rounded-lg border-l-4 bg-muted/20 p-4 animate-pulse"
            style={{
              borderLeftColor:
                (agentColors[stream.speakingAgentType ?? ""] ?? { border: "#9e9e9e" }).border,
            }}
          >
            <div className="mb-2 flex items-center gap-2">
              <span
                className="rounded-full px-2 py-0.5 text-xs font-medium"
                style={{
                  backgroundColor:
                    (agentColors[stream.speakingAgentType ?? ""] ?? { bg: "#f5f5f5" }).bg,
                  color:
                    (agentColors[stream.speakingAgentType ?? ""] ?? { text: "#616161" }).text,
                }}
              >
                {stream.speakingAgentId}
              </span>
              <span className="text-muted-foreground text-xs">typing...</span>
            </div>
            {stream.streamingContent && (
              <MarkdownRenderer content={stream.streamingContent} />
            )}
          </div>
        </div>
      )}

      <div ref={bottomRef} />
    </div>
  )
}
