import { Link } from "@tanstack/react-router"
import { useEffect, useState } from "react"

import type { CouncilMeta } from "@/store/council"

const agentColors: Record<string, { bg: string; text: string }> = {
  researcher: { bg: "#e3f2fd", text: "#1565c0" },
  coder: { bg: "#f3e5f5", text: "#7b1fa2" },
  planner: { bg: "#fff3e0", text: "#e65100" },
  reviewer: { bg: "#e8f5e9", text: "#2e7d32" },
}

const statusStyles: Record<string, { color: string; label: string }> = {
  active: { color: "bg-green-500", label: "Active" },
  paused: { color: "bg-orange-400", label: "Paused" },
  closed: { color: "bg-zinc-400", label: "Closed" },
}

export function CouncilList() {
  const [councils, setCouncils] = useState<CouncilMeta[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    fetch("/api/councils")
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`)
        return res.json()
      })
      .then((data: CouncilMeta[]) => {
        setCouncils(data)
        setLoading(false)
      })
      .catch((err) => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-muted-foreground animate-pulse text-sm">
          Loading councils...
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-destructive text-sm">
          Failed to load councils: {error}
        </div>
      </div>
    )
  }

  if (councils.length === 0) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-muted-foreground text-sm">
          No council sessions yet
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-4xl p-6">
      <h1 className="mb-6 text-2xl font-semibold">Councils</h1>
      <div className="flex flex-col gap-3">
        {councils.map((council) => {
          const status = statusStyles[council.status] ?? statusStyles.closed
          return (
            <Link
              key={council.id}
              to="/councils/$id"
              params={{ id: council.id }}
              className="border-border/40 hover:border-border hover:bg-muted/40 flex items-start gap-4 rounded-lg border p-4 transition-colors"
            >
              <div className="mt-1.5 flex-shrink-0">
                <div
                  className={`h-2.5 w-2.5 rounded-full ${status.color}`}
                  title={status.label}
                />
              </div>
              <div className="min-w-0 flex-1">
                <div className="mb-1 flex items-center gap-3">
                  <span className="truncate font-medium">{council.title}</span>
                  <span className="text-muted-foreground flex-shrink-0 text-xs">
                    {council.rounds} round{council.rounds !== 1 ? "s" : ""}
                  </span>
                </div>
                {council.description && (
                  <p className="text-muted-foreground mb-2 line-clamp-2 text-sm">
                    {council.description}
                  </p>
                )}
                <div className="flex flex-wrap gap-1.5">
                  {(council.roster ?? []).map((agent) => {
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
            </Link>
          )
        })}
      </div>
    </div>
  )
}
