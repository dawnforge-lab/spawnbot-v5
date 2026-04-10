import { IconDeviceFloppy } from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { patchAppConfig } from "@/api/channels"
import { Field, SwitchCardField } from "@/components/shared-form"
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

// ── Form ────────────────────────────────────────────────────

interface SystemForm {
  // Hooks
  hooksEnabled: boolean
  hooksObserverTimeout: string
  hooksInterceptorTimeout: string
  hooksApprovalTimeout: string
  // Autonomy
  autonomyIdleEnabled: boolean
  autonomyIdleThresholdHours: string
  // Voice
  voiceEchoTranscription: boolean
  // Embeddings
  embeddingsProvider: string
  embeddingsModel: string
  embeddingsBaseUrl: string
  embeddingsDimensions: string
  // Self-improve
  selfImproveEnabled: boolean
  selfImproveHour: string
  selfImproveMaxCreations: string
  selfImproveMaxRetries: string
  // Gateway
  gatewayHost: string
  gatewayPort: string
  gatewayHotReload: boolean
  gatewayLogLevel: string
}

// ── Helpers ─────────────────────────────────────────────────

type JsonRecord = Record<string, unknown>

function asRecord(v: unknown): JsonRecord {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as JsonRecord) : {}
}

function asBool(v: unknown, fb = false): boolean {
  return v === undefined ? fb : v === true
}

function asNumStr(v: unknown, fb: string): string {
  if (typeof v === "number" && Number.isFinite(v)) return String(v)
  if (typeof v === "string" && v.trim() !== "") return v
  return fb
}

function asStr(v: unknown, fb = ""): string {
  return typeof v === "string" ? v : fb
}

function parseInt_(raw: string, label: string, opts: { min?: number; max?: number } = {}): number {
  const v = Number(raw)
  if (!Number.isInteger(v)) throw new Error(`${label} must be an integer.`)
  if (opts.min !== undefined && v < opts.min) throw new Error(`${label} must be >= ${opts.min}.`)
  if (opts.max !== undefined && v > opts.max) throw new Error(`${label} must be <= ${opts.max}.`)
  return v
}

const EMPTY: SystemForm = {
  hooksEnabled: true,
  hooksObserverTimeout: "500",
  hooksInterceptorTimeout: "5000",
  hooksApprovalTimeout: "60000",
  autonomyIdleEnabled: false,
  autonomyIdleThresholdHours: "8",
  voiceEchoTranscription: false,
  embeddingsProvider: "",
  embeddingsModel: "",
  embeddingsBaseUrl: "",
  embeddingsDimensions: "768",
  selfImproveEnabled: false,
  selfImproveHour: "3",
  selfImproveMaxCreations: "10",
  selfImproveMaxRetries: "5",
  gatewayHost: "127.0.0.1",
  gatewayPort: "18790",
  gatewayHotReload: false,
  gatewayLogLevel: "info",
}

function buildForm(config: unknown): SystemForm {
  const root = asRecord(config)
  const hooks = asRecord(root.hooks)
  const hooksDef = asRecord(hooks.defaults)
  const autonomy = asRecord(root.autonomy)
  const idle = asRecord(autonomy.idle_trigger)
  const voice = asRecord(root.voice)
  const emb = asRecord(root.embeddings)
  const si = asRecord(root.self_improve)
  const gw = asRecord(root.gateway)

  return {
    hooksEnabled: asBool(hooks.enabled, true),
    hooksObserverTimeout: asNumStr(hooksDef.observer_timeout_ms, "500"),
    hooksInterceptorTimeout: asNumStr(hooksDef.interceptor_timeout_ms, "5000"),
    hooksApprovalTimeout: asNumStr(hooksDef.approval_timeout_ms, "60000"),
    autonomyIdleEnabled: asBool(idle.enabled, false),
    autonomyIdleThresholdHours: asNumStr(idle.threshold_hours, "8"),
    voiceEchoTranscription: asBool(voice.echo_transcription, false),
    embeddingsProvider: asStr(emb.provider),
    embeddingsModel: asStr(emb.model),
    embeddingsBaseUrl: asStr(emb.base_url),
    embeddingsDimensions: asNumStr(emb.dimensions, "768"),
    selfImproveEnabled: asBool(si.enabled, false),
    selfImproveHour: asNumStr(si.hour, "3"),
    selfImproveMaxCreations: asNumStr(si.max_creations, "10"),
    selfImproveMaxRetries: asNumStr(si.max_retries, "5"),
    gatewayHost: asStr(gw.host, "127.0.0.1"),
    gatewayPort: asNumStr(gw.port, "18790"),
    gatewayHotReload: asBool(gw.hot_reload, false),
    gatewayLogLevel: asStr(gw.log_level, "info"),
  }
}

function SectionCard({ title, description, children }: { title: string; description?: string; children: React.ReactNode }) {
  return (
    <Card size="sm">
      <CardHeader className="border-border border-b">
        <CardTitle>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent className="pt-0">
        <div className="divide-border/70 divide-y">{children}</div>
      </CardContent>
    </Card>
  )
}

// ── Page ────────────────────────────────────────────────────

export function SystemSettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<SystemForm>(EMPTY)
  const [baseline, setBaseline] = useState<SystemForm>(EMPTY)
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
    const parsed = buildForm(data)
    setForm(parsed)
    setBaseline(parsed)
  }, [data])

  const isDirty = JSON.stringify(form) !== JSON.stringify(baseline)

  const up = <K extends keyof SystemForm>(key: K, value: SystemForm[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const handleReset = () => {
    setForm(baseline)
    toast.info(t("pages.config.reset_success"))
  }

  const handleSave = async () => {
    try {
      setSaving(true)

      await patchAppConfig({
        hooks: {
          enabled: form.hooksEnabled,
          defaults: {
            observer_timeout_ms: parseInt_(form.hooksObserverTimeout, "Observer timeout", { min: 100 }),
            interceptor_timeout_ms: parseInt_(form.hooksInterceptorTimeout, "Interceptor timeout", { min: 100 }),
            approval_timeout_ms: parseInt_(form.hooksApprovalTimeout, "Approval timeout", { min: 1000 }),
          },
        },
        autonomy: {
          idle_trigger: {
            enabled: form.autonomyIdleEnabled,
            threshold_hours: parseInt_(form.autonomyIdleThresholdHours, "Idle threshold", { min: 1 }),
          },
        },
        voice: {
          echo_transcription: form.voiceEchoTranscription,
        },
        embeddings: {
          provider: form.embeddingsProvider,
          model: form.embeddingsModel,
          base_url: form.embeddingsBaseUrl,
          dimensions: parseInt_(form.embeddingsDimensions, "Dimensions", { min: 1 }),
        },
        self_improve: {
          enabled: form.selfImproveEnabled,
          hour: parseInt_(form.selfImproveHour, "Hour", { min: 0, max: 23 }),
          max_creations: parseInt_(form.selfImproveMaxCreations, "Max creations", { min: 1 }),
          max_retries: parseInt_(form.selfImproveMaxRetries, "Max retries", { min: 1 }),
        },
        gateway: {
          host: form.gatewayHost,
          port: parseInt_(form.gatewayPort, "Gateway port", { min: 1, max: 65535 }),
          hot_reload: form.gatewayHotReload,
          log_level: form.gatewayLogLevel,
        },
      })

      setBaseline(form)
      queryClient.invalidateQueries({ queryKey: ["config"] })
      toast.success(t("pages.config.save_success"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("pages.config.save_error"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title="System Settings" />
      <div className="flex-1 overflow-auto p-3 lg:p-6">
        <div className="mx-auto w-full max-w-[1000px] space-y-6">
          {isLoading ? (
            <div className="text-muted-foreground py-6 text-sm">{t("labels.loading")}</div>
          ) : error ? (
            <div className="text-destructive py-6 text-sm">{t("pages.config.load_error")}</div>
          ) : (
            <div className="space-y-6">
              {isDirty && (
                <div className="bg-yellow-50 px-3 py-2 text-sm text-yellow-700">{t("pages.config.unsaved_changes")}</div>
              )}

              {/* Gateway */}
              <SectionCard title="Gateway" description="Gateway process configuration. Changes require gateway restart.">
                <Field label="Host" hint="IP address the gateway listens on." layout="setting-row">
                  <Input value={form.gatewayHost} onChange={(e) => up("gatewayHost", e.target.value)} />
                </Field>
                <Field label="Port" hint="Port the gateway listens on." layout="setting-row">
                  <Input type="number" min={1} max={65535} value={form.gatewayPort} onChange={(e) => up("gatewayPort", e.target.value)} />
                </Field>
                <SwitchCardField label="Hot Reload" hint="Reload config without restart." layout="setting-row" checked={form.gatewayHotReload} onCheckedChange={(v) => up("gatewayHotReload", v)} />
                <Field label="Log Level" layout="setting-row">
                  <Select value={form.gatewayLogLevel} onValueChange={(v) => up("gatewayLogLevel", v)}>
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="debug">Debug</SelectItem>
                      <SelectItem value="info">Info</SelectItem>
                      <SelectItem value="warn">Warn</SelectItem>
                      <SelectItem value="error">Error</SelectItem>
                    </SelectContent>
                  </Select>
                </Field>
              </SectionCard>

              {/* Hooks */}
              <SectionCard title="Hooks" description="Lifecycle hooks for tool calls and turn events.">
                <SwitchCardField label="Enabled" hint="Enable the hooks system." layout="setting-row" checked={form.hooksEnabled} onCheckedChange={(v) => up("hooksEnabled", v)} />
                {form.hooksEnabled && (
                  <>
                    <Field label="Observer Timeout (ms)" hint="Timeout for fire-and-forget hooks." layout="setting-row">
                      <Input type="number" min={100} value={form.hooksObserverTimeout} onChange={(e) => up("hooksObserverTimeout", e.target.value)} />
                    </Field>
                    <Field label="Interceptor Timeout (ms)" hint="Timeout for blocking hooks." layout="setting-row">
                      <Input type="number" min={100} value={form.hooksInterceptorTimeout} onChange={(e) => up("hooksInterceptorTimeout", e.target.value)} />
                    </Field>
                    <Field label="Approval Timeout (ms)" hint="Timeout for user approval hooks." layout="setting-row">
                      <Input type="number" min={1000} value={form.hooksApprovalTimeout} onChange={(e) => up("hooksApprovalTimeout", e.target.value)} />
                    </Field>
                  </>
                )}
              </SectionCard>

              {/* Embeddings */}
              <SectionCard title="Embeddings" description="Vector embedding provider for semantic search and memory.">
                <Field label="Provider" hint="Embedding provider name." layout="setting-row">
                  <Input value={form.embeddingsProvider} onChange={(e) => up("embeddingsProvider", e.target.value)} placeholder="e.g. gemini, openai" />
                </Field>
                <Field label="Model" hint="Embedding model name." layout="setting-row">
                  <Input value={form.embeddingsModel} onChange={(e) => up("embeddingsModel", e.target.value)} placeholder="e.g. gemini-embedding-2-preview" />
                </Field>
                <Field label="Base URL" hint="API endpoint for the embedding provider." layout="setting-row">
                  <Input value={form.embeddingsBaseUrl} onChange={(e) => up("embeddingsBaseUrl", e.target.value)} placeholder="https://..." />
                </Field>
                <Field label="Dimensions" hint="Output vector dimensions." layout="setting-row">
                  <Input type="number" min={1} value={form.embeddingsDimensions} onChange={(e) => up("embeddingsDimensions", e.target.value)} />
                </Field>
              </SectionCard>

              {/* Autonomy */}
              <SectionCard title="Autonomy" description="Autonomous agent behavior when idle.">
                <SwitchCardField label="Idle Trigger" hint="Agent acts autonomously after being idle." layout="setting-row" checked={form.autonomyIdleEnabled} onCheckedChange={(v) => up("autonomyIdleEnabled", v)} />
                {form.autonomyIdleEnabled && (
                  <Field label="Idle Threshold (hours)" hint="Hours of inactivity before autonomous action." layout="setting-row">
                    <Input type="number" min={1} value={form.autonomyIdleThresholdHours} onChange={(e) => up("autonomyIdleThresholdHours", e.target.value)} />
                  </Field>
                )}
              </SectionCard>

              {/* Self-Improve */}
              <SectionCard title="Self-Improve" description="Automated skill creation and improvement.">
                <SwitchCardField label="Enabled" hint="Allow the agent to create and improve skills automatically." layout="setting-row" checked={form.selfImproveEnabled} onCheckedChange={(v) => up("selfImproveEnabled", v)} />
                {form.selfImproveEnabled && (
                  <>
                    <Field label="Hour" hint="Hour of day (0-23) to run self-improvement." layout="setting-row">
                      <Input type="number" min={0} max={23} value={form.selfImproveHour} onChange={(e) => up("selfImproveHour", e.target.value)} />
                    </Field>
                    <Field label="Max Creations" hint="Maximum skills to create per run." layout="setting-row">
                      <Input type="number" min={1} value={form.selfImproveMaxCreations} onChange={(e) => up("selfImproveMaxCreations", e.target.value)} />
                    </Field>
                    <Field label="Max Retries" hint="Maximum retries per skill creation." layout="setting-row">
                      <Input type="number" min={1} value={form.selfImproveMaxRetries} onChange={(e) => up("selfImproveMaxRetries", e.target.value)} />
                    </Field>
                  </>
                )}
              </SectionCard>

              {/* Voice */}
              <SectionCard title="Voice" description="Voice input settings.">
                <SwitchCardField label="Echo Transcription" hint="Echo voice transcription text back to chat." layout="setting-row" checked={form.voiceEchoTranscription} onCheckedChange={(v) => up("voiceEchoTranscription", v)} />
              </SectionCard>

              {/* Save/Reset */}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={handleReset} disabled={!isDirty || saving}>{t("common.reset")}</Button>
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
