export interface PendingContinuation {
  id: string
  agent_id: string
  session_key: string
  kind: "wait" | "schedule" | "await_event" | string
  intent: string
  created_at: string
  fires_at?: string
  event_name?: string
}

export async function getPendingContinuations(agentID: string): Promise<PendingContinuation[]> {
  const res = await fetch(`/api/agents/${encodeURIComponent(agentID)}/continuations`)
  if (!res.ok) return []
  return res.json()
}
