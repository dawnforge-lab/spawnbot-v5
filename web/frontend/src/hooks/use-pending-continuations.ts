import { useEffect, useState } from "react"

import { getPendingContinuations, type PendingContinuation } from "@/api/continuations"

export function usePendingContinuations(agentID: string, pollIntervalMs = 5000) {
  const [pending, setPending] = useState<PendingContinuation[]>([])

  useEffect(() => {
    if (!agentID) return
    let cancelled = false

    const poll = async () => {
      try {
        const result = await getPendingContinuations(agentID)
        if (!cancelled) setPending(result)
      } catch {
        // gateway may not be running; keep previous state
      }
    }

    poll()
    const id = setInterval(poll, pollIntervalMs)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [agentID, pollIntervalMs])

  return pending
}
