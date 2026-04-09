import { atom } from "jotai"

export interface CouncilMeta {
  id: string
  title: string
  description: string
  roster: string[]
  status: "active" | "paused" | "closed"
  rounds: number
  created_at: string
  updated_at: string
}

export interface TranscriptEntry {
  role: "system" | "agent" | "moderator" | "user" | "synthesis"
  agent_id?: string
  agent_type?: string
  content: string
  round: number
  ts: string
}

export interface CouncilDetail extends CouncilMeta {
  transcript: TranscriptEntry[]
}

export interface CouncilStreamState {
  activeCouncilId: string | null
  speakingAgentId: string | null
  speakingAgentType: string | null
  streamingContent: string
  currentRound: number
}

export const councilListAtom = atom<CouncilMeta[]>([])
export const councilDetailAtom = atom<CouncilDetail | null>(null)
export const councilStreamAtom = atom<CouncilStreamState>({
  activeCouncilId: null,
  speakingAgentId: null,
  speakingAgentType: null,
  streamingContent: "",
  currentRound: 0,
})
