import {
  IconDeviceFloppy,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { patchAppConfig } from "@/api/channels"
import { Field } from "@/components/shared-form"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

// ── Types ───────────────────────────────────────────────────

interface AgentForm {
  id: string
  name: string
  isDefault: boolean
  workspace: string
  modelPrimary: string
  modelFallbacks: string
  skills: string
  subagentAllowAgents: string
  subagentModelPrimary: string
}

interface BindingForm {
  agentId: string
  channel: string
  accountId: string
  peerId: string
  guildId: string
}

// ── Helpers ─────────────────────────────────────────────────

type JsonRecord = Record<string, unknown>

function asRecord(value: unknown): JsonRecord {
  if (value && typeof value === "object" && !Array.isArray(value)) return value as JsonRecord
  return {}
}

function asStr(value: unknown): string {
  return typeof value === "string" ? value : ""
}

function asBool(value: unknown): boolean {
  return value === true
}

function asArr(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((v): v is string => typeof v === "string") : []
}

function emptyAgent(): AgentForm {
  return {
    id: "",
    name: "",
    isDefault: false,
    workspace: "",
    modelPrimary: "",
    modelFallbacks: "",
    skills: "",
    subagentAllowAgents: "",
    subagentModelPrimary: "",
  }
}

function emptyBinding(): BindingForm {
  return { agentId: "", channel: "", accountId: "", peerId: "", guildId: "" }
}

function buildAgentsFromConfig(config: unknown): { agents: AgentForm[]; bindings: BindingForm[] } {
  const root = asRecord(config)
  const agentsConfig = asRecord(root.agents)
  const list = Array.isArray(agentsConfig.list) ? agentsConfig.list : []
  const bindingsRaw = Array.isArray(root.bindings) ? root.bindings : []

  const agents: AgentForm[] = list.map((a: unknown) => {
    const ag = asRecord(a)
    const model = asRecord(ag.model)
    const sub = asRecord(ag.subagents)
    const subModel = asRecord(sub.model)
    return {
      id: asStr(ag.id),
      name: asStr(ag.name),
      isDefault: asBool(ag.default),
      workspace: asStr(ag.workspace),
      modelPrimary: asStr(model.primary),
      modelFallbacks: asArr(model.fallbacks).join(", "),
      skills: asArr(ag.skills).join(", "),
      subagentAllowAgents: asArr(sub.allow_agents).join(", "),
      subagentModelPrimary: asStr(subModel.primary),
    }
  })

  const bindings: BindingForm[] = bindingsRaw.map((b: unknown) => {
    const br = asRecord(b)
    const match = asRecord(br.match)
    const peer = asRecord(match.peer)
    return {
      agentId: asStr(br.agent_id),
      channel: asStr(match.channel),
      accountId: asStr(match.account_id),
      peerId: asStr(peer.id),
      guildId: asStr(match.guild_id),
    }
  })

  return { agents, bindings }
}

function splitComma(s: string): string[] {
  return s.split(",").map((v) => v.trim()).filter((v) => v.length > 0)
}

// ── Component ───────────────────────────────────────────────

export function AgentsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [agents, setAgents] = useState<AgentForm[]>([])
  const [bindings, setBindings] = useState<BindingForm[]>([])
  const [baseline, setBaseline] = useState<string>("")
  const [saving, setSaving] = useState(false)

  const { data, isLoading, error } = useQuery({
    queryKey: ["config"],
    queryFn: async () => {
      const res = await fetch("/api/config")
      if (!res.ok) throw new Error("Failed to load config")
      return res.json()
    },
  })

  useEffect(() => {
    if (!data) return
    const parsed = buildAgentsFromConfig(data)
    setAgents(parsed.agents)
    setBindings(parsed.bindings)
    setBaseline(JSON.stringify(parsed))
  }, [data])

  const isDirty = JSON.stringify({ agents, bindings }) !== baseline

  const updateAgent = (index: number, field: keyof AgentForm, value: string | boolean) => {
    setAgents((prev) => {
      const next = [...prev]
      next[index] = { ...next[index], [field]: value }
      return next
    })
  }

  const addAgent = () => {
    setAgents((prev) => [...prev, emptyAgent()])
  }

  const removeAgent = (index: number) => {
    setAgents((prev) => prev.filter((_, i) => i !== index))
  }

  const updateBinding = (index: number, field: keyof BindingForm, value: string) => {
    setBindings((prev) => {
      const next = [...prev]
      next[index] = { ...next[index], [field]: value }
      return next
    })
  }

  const addBinding = () => {
    setBindings((prev) => [...prev, emptyBinding()])
  }

  const removeBinding = (index: number) => {
    setBindings((prev) => prev.filter((_, i) => i !== index))
  }

  const handleReset = () => {
    if (!data) return
    const parsed = buildAgentsFromConfig(data)
    setAgents(parsed.agents)
    setBindings(parsed.bindings)
    toast.info(t("pages.config.reset_success"))
  }

  const handleSave = async () => {
    try {
      setSaving(true)

      // Validate agents
      for (const agent of agents) {
        if (!agent.id.trim()) throw new Error("All agents must have an ID.")
        if (!/^[a-z0-9_-]+$/i.test(agent.id.trim())) {
          throw new Error(`Agent ID "${agent.id}" must be alphanumeric with hyphens/underscores.`)
        }
      }

      // Check for duplicate IDs
      const ids = agents.map((a) => a.id.trim().toLowerCase())
      const uniqueIds = new Set(ids)
      if (uniqueIds.size !== ids.length) {
        throw new Error("Agent IDs must be unique.")
      }

      // Validate bindings
      for (const binding of bindings) {
        if (!binding.agentId.trim()) throw new Error("All bindings must have an Agent ID.")
        if (!binding.channel.trim()) throw new Error("All bindings must have a Channel.")
      }

      const agentList = agents.map((a) => {
        const entry: JsonRecord = {
          id: a.id.trim(),
          default: a.isDefault,
        }
        if (a.name.trim()) entry.name = a.name.trim()
        if (a.workspace.trim()) entry.workspace = a.workspace.trim()
        if (a.modelPrimary.trim()) {
          const model: JsonRecord = { primary: a.modelPrimary.trim() }
          const fallbacks = splitComma(a.modelFallbacks)
          if (fallbacks.length > 0) model.fallbacks = fallbacks
          entry.model = model
        }
        const skills = splitComma(a.skills)
        if (skills.length > 0) entry.skills = skills
        if (a.subagentAllowAgents.trim() || a.subagentModelPrimary.trim()) {
          const sub: JsonRecord = {}
          const allow = splitComma(a.subagentAllowAgents)
          if (allow.length > 0) sub.allow_agents = allow
          if (a.subagentModelPrimary.trim()) {
            sub.model = { primary: a.subagentModelPrimary.trim() }
          }
          entry.subagents = sub
        }
        return entry
      })

      const bindingsList = bindings.map((b) => {
        const match: JsonRecord = { channel: b.channel.trim() }
        if (b.accountId.trim()) match.account_id = b.accountId.trim()
        if (b.peerId.trim()) match.peer = { kind: "user", id: b.peerId.trim() }
        if (b.guildId.trim()) match.guild_id = b.guildId.trim()
        return { agent_id: b.agentId.trim(), match }
      })

      await patchAppConfig({
        agents: {
          list: agentList,
        },
        bindings: bindingsList,
      })

      setBaseline(JSON.stringify({ agents, bindings }))
      queryClient.invalidateQueries({ queryKey: ["config"] })
      toast.success(t("pages.config.save_success"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("pages.config.save_error"))
    } finally {
      setSaving(false)
    }
  }

  const channelOptions = ["telegram", "discord", "slack", "whatsapp", "matrix", "pico", "irc", "feishu", "line", "qq", "dingtalk", "onebot", "wecom", "weixin", "maixcam"]

  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Agent Definitions" />
      <div className="flex-1 overflow-auto p-3 lg:p-6">
        <div className="mx-auto w-full max-w-[1000px] space-y-6">
          {isLoading ? (
            <div className="text-muted-foreground py-6 text-sm">{t("labels.loading")}</div>
          ) : error ? (
            <div className="text-destructive py-6 text-sm">{t("pages.config.load_error")}</div>
          ) : (
            <div className="space-y-6">
              {isDirty && (
                <div className="bg-yellow-50 px-3 py-2 text-sm text-yellow-700">
                  {t("pages.config.unsaved_changes")}
                </div>
              )}

              {/* Info */}
              {agents.length === 0 && (
                <Card size="sm">
                  <CardContent className="py-6">
                    <p className="text-muted-foreground text-sm">
                      No custom agents defined. Spawnbot uses a single implicit &quot;main&quot; agent with the defaults from Agent Config.
                      Add agents here to create specialized agents with their own models, skills, and channel routing.
                    </p>
                  </CardContent>
                </Card>
              )}

              {/* Agent Cards */}
              {agents.map((agent, i) => (
                <Card key={i} size="sm">
                  <CardHeader className="border-border border-b">
                    <div className="flex items-center justify-between">
                      <CardTitle>{agent.id || "New Agent"}</CardTitle>
                      <Button variant="ghost" size="sm" onClick={() => removeAgent(i)}>
                        <IconTrash className="text-destructive size-4" />
                      </Button>
                    </div>
                    {agent.name && <CardDescription>{agent.name}</CardDescription>}
                  </CardHeader>
                  <CardContent className="pt-0">
                    <div className="divide-border/70 divide-y">
                      <Field label="Agent ID" hint="Unique identifier (alphanumeric, hyphens, underscores)." layout="setting-row">
                        <Input value={agent.id} onChange={(e) => updateAgent(i, "id", e.target.value)} placeholder="e.g. researcher" />
                      </Field>
                      <Field label="Display Name" hint="Human-readable name." layout="setting-row">
                        <Input value={agent.name} onChange={(e) => updateAgent(i, "name", e.target.value)} placeholder="e.g. Research Agent" />
                      </Field>
                      <div className="flex items-center justify-between py-4">
                        <div className="space-y-1">
                          <div className="text-sm font-medium">Default Agent</div>
                          <div className="text-muted-foreground text-xs">Handle messages when no binding matches.</div>
                        </div>
                        <Switch checked={agent.isDefault} onCheckedChange={(v) => updateAgent(i, "isDefault", v)} />
                      </div>
                      <Field label="Model" hint="Primary model for this agent. Leave empty to use defaults." layout="setting-row">
                        <Input value={agent.modelPrimary} onChange={(e) => updateAgent(i, "modelPrimary", e.target.value)} placeholder="e.g. gemma4:31b" />
                      </Field>
                      <Field label="Fallback Models" hint="Comma-separated fallback model names." layout="setting-row">
                        <Input value={agent.modelFallbacks} onChange={(e) => updateAgent(i, "modelFallbacks", e.target.value)} placeholder="e.g. gpt-5.4, claude-sonnet-4.6" />
                      </Field>
                      <Field label="Workspace" hint="Custom workspace path. Leave empty for default." layout="setting-row">
                        <Input value={agent.workspace} onChange={(e) => updateAgent(i, "workspace", e.target.value)} placeholder="/path/to/workspace" />
                      </Field>
                      <Field label="Skills" hint="Comma-separated skill names this agent can use." layout="setting-row">
                        <Input value={agent.skills} onChange={(e) => updateAgent(i, "skills", e.target.value)} placeholder="e.g. weather, code-review" />
                      </Field>
                      <Field label="Allowed Subagents" hint="Comma-separated agent IDs this agent can spawn as subagents." layout="setting-row">
                        <Input value={agent.subagentAllowAgents} onChange={(e) => updateAgent(i, "subagentAllowAgents", e.target.value)} placeholder="e.g. coder, researcher" />
                      </Field>
                      <Field label="Subagent Model" hint="Override model for subagents spawned by this agent." layout="setting-row">
                        <Input value={agent.subagentModelPrimary} onChange={(e) => updateAgent(i, "subagentModelPrimary", e.target.value)} placeholder="Leave empty for default" />
                      </Field>
                    </div>
                  </CardContent>
                </Card>
              ))}

              <Button variant="outline" onClick={addAgent} className="w-full">
                <IconPlus className="size-4" />
                Add Agent
              </Button>

              {/* Bindings */}
              <Card size="sm">
                <CardHeader className="border-border border-b">
                  <CardTitle>Channel Bindings</CardTitle>
                  <CardDescription>
                    Route messages from specific channels to specific agents. Messages without a matching binding go to the default agent.
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-0">
                  {bindings.length === 0 ? (
                    <p className="text-muted-foreground py-4 text-sm">
                      No bindings configured. All messages route to the default agent.
                    </p>
                  ) : (
                    <div className="divide-border/70 divide-y">
                      {bindings.map((binding, i) => (
                        <div key={i} className="flex items-start gap-3 py-4">
                          <div className="grid flex-1 grid-cols-2 gap-3 md:grid-cols-4">
                            <div>
                              <label className="text-xs font-medium">Agent ID</label>
                              <Input value={binding.agentId} onChange={(e) => updateBinding(i, "agentId", e.target.value)} placeholder="agent-id" className="mt-1" />
                            </div>
                            <div>
                              <label className="text-xs font-medium">Channel</label>
                              <Select value={binding.channel} onValueChange={(v) => updateBinding(i, "channel", v)}>
                                <SelectTrigger className="mt-1">
                                  <SelectValue placeholder="Select" />
                                </SelectTrigger>
                                <SelectContent>
                                  {channelOptions.map((ch) => (
                                    <SelectItem key={ch} value={ch}>{ch}</SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </div>
                            <div>
                              <label className="text-xs font-medium">Peer ID</label>
                              <Input value={binding.peerId} onChange={(e) => updateBinding(i, "peerId", e.target.value)} placeholder="Optional" className="mt-1" />
                            </div>
                            <div>
                              <label className="text-xs font-medium">Guild/Group ID</label>
                              <Input value={binding.guildId} onChange={(e) => updateBinding(i, "guildId", e.target.value)} placeholder="Optional" className="mt-1" />
                            </div>
                          </div>
                          <Button variant="ghost" size="sm" onClick={() => removeBinding(i)} className="mt-5">
                            <IconTrash className="text-destructive size-4" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  )}
                  <Button variant="outline" onClick={addBinding} className="mt-3 w-full" size="sm">
                    <IconPlus className="size-4" />
                    Add Binding
                  </Button>
                </CardContent>
              </Card>

              {/* Save/Reset */}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={handleReset} disabled={!isDirty || saving}>
                  {t("common.reset")}
                </Button>
                <Button onClick={handleSave} disabled={!isDirty || saving}>
                  <IconDeviceFloppy className="size-4" />
                  {saving ? t("common.saving") : t("common.save")}
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
