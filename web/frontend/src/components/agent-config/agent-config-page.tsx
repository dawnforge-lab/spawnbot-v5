import { IconDeviceFloppy } from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { patchAppConfig } from "@/api/channels"
import { type ModelInfo, getModels } from "@/api/models"
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

interface AgentConfigForm {
  mainModel: string
  subturnModel: string
  subturnMaxDepth: string
  subturnMaxConcurrent: string
  subturnDefaultTimeoutMinutes: string
  subturnDefaultTokenBudget: string
  subturnConcurrencyTimeoutSec: string
}

const EMPTY_FORM: AgentConfigForm = {
  mainModel: "",
  subturnModel: "",
  subturnMaxDepth: "3",
  subturnMaxConcurrent: "5",
  subturnDefaultTimeoutMinutes: "5",
  subturnDefaultTokenBudget: "0",
  subturnConcurrencyTimeoutSec: "30",
}

function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return {}
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : ""
}

function asNumberString(value: unknown, fallback: string): string {
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value)
  }
  if (typeof value === "string" && value.trim() !== "") {
    return value
  }
  return fallback
}

function buildFormFromConfig(config: unknown): AgentConfigForm {
  const root = asRecord(config)
  const agents = asRecord(root.agents)
  const defaults = asRecord(agents.defaults)
  const subturn = asRecord(defaults.subturn)

  return {
    mainModel: asString(defaults.model_name) || asString(defaults.provider) || EMPTY_FORM.mainModel,
    subturnModel: asString(subturn.model) || EMPTY_FORM.subturnModel,
    subturnMaxDepth: asNumberString(subturn.max_depth, EMPTY_FORM.subturnMaxDepth),
    subturnMaxConcurrent: asNumberString(subturn.max_concurrent, EMPTY_FORM.subturnMaxConcurrent),
    subturnDefaultTimeoutMinutes: asNumberString(subturn.default_timeout_minutes, EMPTY_FORM.subturnDefaultTimeoutMinutes),
    subturnDefaultTokenBudget: asNumberString(subturn.default_token_budget, EMPTY_FORM.subturnDefaultTokenBudget),
    subturnConcurrencyTimeoutSec: asNumberString(subturn.concurrency_timeout_sec, EMPTY_FORM.subturnConcurrencyTimeoutSec),
  }
}

function parseIntField(
  rawValue: string,
  label: string,
  options: { min?: number; max?: number } = {},
): number {
  const value = Number(rawValue)
  if (!Number.isInteger(value)) {
    throw new Error(`${label} must be an integer.`)
  }
  if (options.min !== undefined && value < options.min) {
    throw new Error(`${label} must be >= ${options.min}.`)
  }
  if (options.max !== undefined && value > options.max) {
    throw new Error(`${label} must be <= ${options.max}.`)
  }
  return value
}

export function AgentConfigPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<AgentConfigForm>(EMPTY_FORM)
  const [baseline, setBaseline] = useState<AgentConfigForm>(EMPTY_FORM)
  const [models, setModels] = useState<ModelInfo[]>([])
  const [saving, setSaving] = useState(false)

  const { data, isLoading, error } = useQuery({
    queryKey: ["config"],
    queryFn: async () => {
      const res = await fetch("/api/config")
      if (!res.ok) {
        throw new Error("Failed to load config")
      }
      return res.json()
    },
  })

  const { data: modelsData } = useQuery({
    queryKey: ["models"],
    queryFn: getModels,
  })

  useEffect(() => {
    if (!data) return
    const parsed = buildFormFromConfig(data)
    setForm(parsed)
    setBaseline(parsed)
  }, [data])

  useEffect(() => {
    if (!modelsData) return
    setModels(modelsData.models)
  }, [modelsData])

  const isDirty = JSON.stringify(form) !== JSON.stringify(baseline)

  const updateField = <K extends keyof AgentConfigForm>(
    key: K,
    value: AgentConfigForm[K],
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const handleReset = () => {
    setForm(baseline)
    toast.info(t("pages.config.reset_success"))
  }

  const handleSave = async () => {
    try {
      setSaving(true)

      if (!form.mainModel.trim()) {
        throw new Error("Main agent model is required.")
      }

      const subturnMaxDepth = parseIntField(form.subturnMaxDepth, "Max depth", { min: 1, max: 10 })
      const subturnMaxConcurrent = parseIntField(form.subturnMaxConcurrent, "Max concurrent", { min: 1, max: 20 })
      const subturnDefaultTimeoutMinutes = parseIntField(form.subturnDefaultTimeoutMinutes, "Default timeout", { min: 1, max: 60 })
      const subturnDefaultTokenBudget = parseIntField(form.subturnDefaultTokenBudget, "Token budget", { min: 0 })
      const subturnConcurrencyTimeoutSec = parseIntField(form.subturnConcurrencyTimeoutSec, "Concurrency timeout", { min: 1, max: 600 })

      await patchAppConfig({
        agents: {
          defaults: {
            model_name: form.mainModel,
            provider: form.mainModel,
            subturn: {
              model: form.subturnModel || undefined,
              max_depth: subturnMaxDepth,
              max_concurrent: subturnMaxConcurrent,
              default_timeout_minutes: subturnDefaultTimeoutMinutes,
              default_token_budget: subturnDefaultTokenBudget,
              concurrency_timeout_sec: subturnConcurrencyTimeoutSec,
            },
          },
        },
      })

      setBaseline(form)
      queryClient.invalidateQueries({ queryKey: ["config"] })
      toast.success(t("pages.config.save_success"))
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : t("pages.config.save_error"),
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Agent Configuration" />
      <div className="flex-1 overflow-auto p-3 lg:p-6">
        <div className="mx-auto w-full max-w-[1000px] space-y-6">
          {isLoading ? (
            <div className="text-muted-foreground py-6 text-sm">
              {t("labels.loading")}
            </div>
          ) : error ? (
            <div className="text-destructive py-6 text-sm">
              {t("pages.config.load_error")}
            </div>
          ) : (
            <div className="space-y-6">
              {isDirty && (
                <div className="bg-yellow-50 px-3 py-2 text-sm text-yellow-700">
                  {t("pages.config.unsaved_changes")}
                </div>
              )}

              <Card size="sm">
                <CardHeader className="border-border border-b">
                  <CardTitle>Main Agent</CardTitle>
                  <CardDescription>
                    The primary model used for the main agent loop.
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-0">
                  <div className="divide-border/70 divide-y">
                    <Field
                      label="Model"
                      hint="The LLM model used for the main agent. This handles direct conversations and orchestrates subagents."
                      layout="setting-row"
                    >
                      <Select
                        value={form.mainModel}
                        onValueChange={(v) => updateField("mainModel", v)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Select a model" />
                        </SelectTrigger>
                        <SelectContent>
                          {models.map((m) => (
                            <SelectItem key={m.model_name} value={m.model_name}>
                              {m.model_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </Field>
                  </div>
                </CardContent>
              </Card>

              <Card size="sm">
                <CardHeader className="border-border border-b">
                  <CardTitle>Subagents</CardTitle>
                  <CardDescription>
                    Configuration for spawned subagents, council discussions, and nested task execution.
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-0">
                  <div className="divide-border/70 divide-y">
                    <Field
                      label="Subagent Model"
                      hint="The LLM model used for subagents, spawn tasks, and council agents. Falls back to the main model if not set."
                      layout="setting-row"
                    >
                      <Select
                        value={form.subturnModel}
                        onValueChange={(v) => updateField("subturnModel", v)}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Same as main model" />
                        </SelectTrigger>
                        <SelectContent>
                          {models.map((m) => (
                            <SelectItem key={m.model_name} value={m.model_name}>
                              {m.model_name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </Field>

                    <Field
                      label="Max Depth"
                      hint="Maximum nesting depth for subagent spawning (1-10)."
                      layout="setting-row"
                    >
                      <Input
                        type="number"
                        min={1}
                        max={10}
                        value={form.subturnMaxDepth}
                        onChange={(e) => updateField("subturnMaxDepth", e.target.value)}
                      />
                    </Field>

                    <Field
                      label="Max Concurrent"
                      hint="Maximum number of subagents running concurrently per parent turn."
                      layout="setting-row"
                    >
                      <Input
                        type="number"
                        min={1}
                        max={20}
                        value={form.subturnMaxConcurrent}
                        onChange={(e) => updateField("subturnMaxConcurrent", e.target.value)}
                      />
                    </Field>

                    <Field
                      label="Default Timeout (minutes)"
                      hint="Maximum execution time for a single subagent turn."
                      layout="setting-row"
                    >
                      <Input
                        type="number"
                        min={1}
                        max={60}
                        value={form.subturnDefaultTimeoutMinutes}
                        onChange={(e) => updateField("subturnDefaultTimeoutMinutes", e.target.value)}
                      />
                    </Field>

                    <Field
                      label="Token Budget"
                      hint="Default token budget per subagent. Set to 0 for unlimited."
                      layout="setting-row"
                    >
                      <Input
                        type="number"
                        min={0}
                        value={form.subturnDefaultTokenBudget}
                        onChange={(e) => updateField("subturnDefaultTokenBudget", e.target.value)}
                      />
                    </Field>

                    <Field
                      label="Concurrency Timeout (seconds)"
                      hint="How long to wait for a concurrency slot before timing out."
                      layout="setting-row"
                    >
                      <Input
                        type="number"
                        min={1}
                        max={600}
                        value={form.subturnConcurrencyTimeoutSec}
                        onChange={(e) => updateField("subturnConcurrencyTimeoutSec", e.target.value)}
                      />
                    </Field>
                  </div>
                </CardContent>
              </Card>

              <div className="flex justify-end gap-2">
                <Button
                  variant="outline"
                  onClick={handleReset}
                  disabled={!isDirty || saving}
                >
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
