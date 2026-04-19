import { IconClock, IconRefresh, IconX } from "@tabler/icons-react"

import type { PendingContinuation } from "@/api/continuations"
import { Button } from "@/components/ui/button"

interface PendingPanelProps {
  agentName: string
  items: PendingContinuation[]
  onClose: () => void
}

function KindIcon({ kind }: { kind: string }) {
  if (kind === "schedule" || kind === "wait") {
    return <IconClock className="text-muted-foreground size-3.5 shrink-0" />
  }
  return <IconRefresh className="text-muted-foreground size-3.5 shrink-0" />
}

function TimeLabel({ item }: { item: PendingContinuation }) {
  if (item.fires_at) {
    const firesAt = new Date(item.fires_at)
    const now = new Date()
    if (firesAt > now) {
      const diffMs = firesAt.getTime() - now.getTime()
      const diffMins = Math.ceil(diffMs / 60000)
      return (
        <span className="text-muted-foreground text-xs">
          fires in {diffMins} min
        </span>
      )
    }
    return <span className="text-muted-foreground text-xs">firing…</span>
  }
  if (item.event_name) {
    return (
      <span className="text-muted-foreground text-xs">
        waiting for <span className="font-mono">{item.event_name}</span>
      </span>
    )
  }
  return <span className="text-muted-foreground text-xs">no deadline</span>
}

export function PendingPanel({ agentName, items, onClose }: PendingPanelProps) {
  return (
    <div className="bg-background border-border fixed bottom-4 left-4 z-50 w-80 rounded-lg border shadow-lg">
      <div className="flex items-center justify-between border-b px-3 py-2">
        <span className="text-sm font-medium">
          Pending Continuations — {agentName}
        </span>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
          <IconX className="size-3.5" />
        </Button>
      </div>
      <div className="divide-border max-h-64 divide-y overflow-y-auto">
        {items.length === 0 ? (
          <p className="text-muted-foreground px-3 py-3 text-xs">No pending continuations.</p>
        ) : (
          items.map((item) => (
            <div key={item.id} className="flex items-start gap-2 px-3 py-2">
              <KindIcon kind={item.kind} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs">{item.intent || "(no intent)"}</p>
                <TimeLabel item={item} />
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
