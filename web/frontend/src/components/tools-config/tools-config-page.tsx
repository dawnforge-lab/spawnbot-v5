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

// ── Form types ──────────────────────────────────────────────

interface ToolsConfigForm {
  // Simple tool toggles
  appendFile: boolean
  editFile: boolean
  listDir: boolean
  readFile: boolean
  writeFile: boolean
  sendFile: boolean
  message: boolean
  findSkills: boolean
  installSkill: boolean
  spawn: boolean
  spawnStatus: boolean
  subagent: boolean
  webFetch: boolean
  i2c: boolean
  spi: boolean

  // Read file settings
  maxReadFileSize: string

  // Web search
  webEnabled: boolean
  webPreferNative: boolean
  webFetchLimitBytes: string
  webFormat: string
  duckduckgoEnabled: boolean
  duckduckgoMaxResults: string
  braveEnabled: boolean
  braveMaxResults: string
  tavilyEnabled: boolean
  tavilyMaxResults: string
  perplexityEnabled: boolean
  perplexityMaxResults: string
  searxngEnabled: boolean
  searxngBaseUrl: string
  searxngMaxResults: string

  // MCP
  mcpEnabled: boolean
  mcpDiscoveryEnabled: boolean
  mcpDiscoveryMaxResults: string

  // Skills
  skillsEnabled: boolean
  skillsMaxConcurrentSearches: string

  // Media cleanup
  mediaCleanupEnabled: boolean
  mediaCleanupMaxAgeMinutes: string
  mediaCleanupIntervalMinutes: string

  // Result persistence
  resultPersistenceEnabled: boolean
  resultPersistenceDefaultMaxChars: string
  resultPersistencePerTurnBudget: string
  resultPersistencePreviewSize: string

  // Wallet
  walletEnabled: boolean
  walletChain: string
  walletMaxSendAmount: string
  walletMaxTradeAmount: string
  walletMaxPayAmount: string

  // Global
  filterSensitiveData: boolean
  filterMinLength: string
}

// ── Helpers ─────────────────────────────────────────────────

type JsonRecord = Record<string, unknown>

function asRecord(value: unknown): JsonRecord {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as JsonRecord
  }
  return {}
}

function asBool(value: unknown, fallback = false): boolean {
  return value === undefined ? fallback : value === true
}

function asNumStr(value: unknown, fallback: string): string {
  if (typeof value === "number" && Number.isFinite(value)) return String(value)
  if (typeof value === "string" && value.trim() !== "") return value
  return fallback
}

function asStr(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback
}

function parseIntField(
  raw: string,
  label: string,
  opts: { min?: number; max?: number } = {},
): number {
  const v = Number(raw)
  if (!Number.isInteger(v)) throw new Error(`${label} must be an integer.`)
  if (opts.min !== undefined && v < opts.min)
    throw new Error(`${label} must be >= ${opts.min}.`)
  if (opts.max !== undefined && v > opts.max)
    throw new Error(`${label} must be <= ${opts.max}.`)
  return v
}

// ── Build form from config ──────────────────────────────────

const EMPTY_FORM: ToolsConfigForm = {
  appendFile: true,
  editFile: true,
  listDir: true,
  readFile: true,
  writeFile: true,
  sendFile: true,
  message: true,
  findSkills: true,
  installSkill: true,
  spawn: true,
  spawnStatus: false,
  subagent: true,
  webFetch: true,
  i2c: false,
  spi: false,
  maxReadFileSize: "65536",
  webEnabled: true,
  webPreferNative: true,
  webFetchLimitBytes: "10485760",
  webFormat: "plaintext",
  duckduckgoEnabled: true,
  duckduckgoMaxResults: "5",
  braveEnabled: false,
  braveMaxResults: "5",
  tavilyEnabled: false,
  tavilyMaxResults: "5",
  perplexityEnabled: false,
  perplexityMaxResults: "5",
  searxngEnabled: false,
  searxngBaseUrl: "",
  searxngMaxResults: "5",
  mcpEnabled: false,
  mcpDiscoveryEnabled: true,
  mcpDiscoveryMaxResults: "5",
  skillsEnabled: true,
  skillsMaxConcurrentSearches: "2",
  mediaCleanupEnabled: true,
  mediaCleanupMaxAgeMinutes: "30",
  mediaCleanupIntervalMinutes: "5",
  resultPersistenceEnabled: true,
  resultPersistenceDefaultMaxChars: "50000",
  resultPersistencePerTurnBudget: "200000",
  resultPersistencePreviewSize: "2000",
  walletEnabled: false,
  walletChain: "base",
  walletMaxSendAmount: "100",
  walletMaxTradeAmount: "50",
  walletMaxPayAmount: "1",
  filterSensitiveData: true,
  filterMinLength: "8",
}

function buildFormFromConfig(config: unknown): ToolsConfigForm {
  const root = asRecord(config)
  const tools = asRecord(root.tools)
  const web = asRecord(tools.web)
  const ddg = asRecord(web.duckduckgo)
  const brave = asRecord(web.brave)
  const tavily = asRecord(web.tavily)
  const perplexity = asRecord(web.perplexity)
  const searxng = asRecord(web.searxng)
  const mcp = asRecord(tools.mcp)
  const mcpDiscovery = asRecord(mcp.discovery)
  const skills = asRecord(tools.skills)
  const mediaCleanup = asRecord(tools.media_cleanup)
  const resultPersistence = asRecord(tools.result_persistence)
  const wallet = asRecord(tools.wallet)
  const readFile = asRecord(tools.read_file)

  return {
    appendFile: asBool(asRecord(tools.append_file).enabled, true),
    editFile: asBool(asRecord(tools.edit_file).enabled, true),
    listDir: asBool(asRecord(tools.list_dir).enabled, true),
    readFile: asBool(readFile.enabled, true),
    writeFile: asBool(asRecord(tools.write_file).enabled, true),
    sendFile: asBool(asRecord(tools.send_file).enabled, true),
    message: asBool(asRecord(tools.message).enabled, true),
    findSkills: asBool(asRecord(tools.find_skills).enabled, true),
    installSkill: asBool(asRecord(tools.install_skill).enabled, true),
    spawn: asBool(asRecord(tools.spawn).enabled, true),
    spawnStatus: asBool(asRecord(tools.spawn_status).enabled, false),
    subagent: asBool(asRecord(tools.subagent).enabled, true),
    webFetch: asBool(asRecord(tools.web_fetch).enabled, true),
    i2c: asBool(asRecord(tools.i2c).enabled, false),
    spi: asBool(asRecord(tools.spi).enabled, false),
    maxReadFileSize: asNumStr(readFile.max_read_file_size, EMPTY_FORM.maxReadFileSize),
    webEnabled: asBool(web.enabled, true),
    webPreferNative: asBool(web.prefer_native, true),
    webFetchLimitBytes: asNumStr(web.fetch_limit_bytes, EMPTY_FORM.webFetchLimitBytes),
    webFormat: asStr(web.format as string, "plaintext"),
    duckduckgoEnabled: asBool(ddg.enabled, true),
    duckduckgoMaxResults: asNumStr(ddg.max_results, "5"),
    braveEnabled: asBool(brave.enabled, false),
    braveMaxResults: asNumStr(brave.max_results, "5"),
    tavilyEnabled: asBool(tavily.enabled, false),
    tavilyMaxResults: asNumStr(tavily.max_results, "5"),
    perplexityEnabled: asBool(perplexity.enabled, false),
    perplexityMaxResults: asNumStr(perplexity.max_results, "5"),
    searxngEnabled: asBool(searxng.enabled, false),
    searxngBaseUrl: asStr(searxng.base_url as string, ""),
    searxngMaxResults: asNumStr(searxng.max_results, "5"),
    mcpEnabled: asBool(mcp.enabled, false),
    mcpDiscoveryEnabled: asBool(mcpDiscovery.enabled, true),
    mcpDiscoveryMaxResults: asNumStr(mcpDiscovery.max_search_results, "5"),
    skillsEnabled: asBool(skills.enabled, true),
    skillsMaxConcurrentSearches: asNumStr(skills.max_concurrent_searches, "2"),
    mediaCleanupEnabled: asBool(mediaCleanup.enabled, true),
    mediaCleanupMaxAgeMinutes: asNumStr(mediaCleanup.max_age_minutes, "30"),
    mediaCleanupIntervalMinutes: asNumStr(mediaCleanup.interval_minutes, "5"),
    resultPersistenceEnabled: asBool(resultPersistence.enabled, true),
    resultPersistenceDefaultMaxChars: asNumStr(resultPersistence.default_max_chars, "50000"),
    resultPersistencePerTurnBudget: asNumStr(resultPersistence.per_turn_budget_chars, "200000"),
    resultPersistencePreviewSize: asNumStr(resultPersistence.preview_size_bytes, "2000"),
    walletEnabled: asBool(wallet.enabled, false),
    walletChain: asStr(wallet.chain as string, "base"),
    walletMaxSendAmount: asNumStr(wallet.max_send_amount, "100"),
    walletMaxTradeAmount: asNumStr(wallet.max_trade_amount, "50"),
    walletMaxPayAmount: asNumStr(wallet.max_pay_amount, "1"),
    filterSensitiveData: asBool(tools.filter_sensitive_data, true),
    filterMinLength: asNumStr(tools.filter_min_length, "8"),
  }
}

// ── Section Components ──────────────────────────────────────

function ConfigSectionCard({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
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

// ── Main Page Component ─────────────────────────────────────

export function ToolsConfigPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [form, setForm] = useState<ToolsConfigForm>(EMPTY_FORM)
  const [baseline, setBaseline] = useState<ToolsConfigForm>(EMPTY_FORM)
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
    const parsed = buildFormFromConfig(data)
    setForm(parsed)
    setBaseline(parsed)
  }, [data])

  const isDirty = JSON.stringify(form) !== JSON.stringify(baseline)

  const update = <K extends keyof ToolsConfigForm>(
    key: K,
    value: ToolsConfigForm[K],
  ) => setForm((prev) => ({ ...prev, [key]: value }))

  const handleReset = () => {
    setForm(baseline)
    toast.info(t("pages.config.reset_success"))
  }

  const handleSave = async () => {
    try {
      setSaving(true)

      await patchAppConfig({
        tools: {
          filter_sensitive_data: form.filterSensitiveData,
          filter_min_length: parseIntField(form.filterMinLength, "Filter min length", { min: 0 }),
          append_file: { enabled: form.appendFile },
          edit_file: { enabled: form.editFile },
          list_dir: { enabled: form.listDir },
          read_file: {
            enabled: form.readFile,
            max_read_file_size: parseIntField(form.maxReadFileSize, "Max read file size", { min: 1024 }),
          },
          write_file: { enabled: form.writeFile },
          send_file: { enabled: form.sendFile },
          message: { enabled: form.message },
          find_skills: { enabled: form.findSkills },
          install_skill: { enabled: form.installSkill },
          spawn: { enabled: form.spawn },
          spawn_status: { enabled: form.spawnStatus },
          subagent: { enabled: form.subagent },
          web_fetch: { enabled: form.webFetch },
          i2c: { enabled: form.i2c },
          spi: { enabled: form.spi },
          web: {
            enabled: form.webEnabled,
            prefer_native: form.webPreferNative,
            fetch_limit_bytes: parseIntField(form.webFetchLimitBytes, "Fetch limit", { min: 0 }),
            format: form.webFormat,
            duckduckgo: {
              enabled: form.duckduckgoEnabled,
              max_results: parseIntField(form.duckduckgoMaxResults, "DuckDuckGo max results", { min: 1, max: 20 }),
            },
            brave: {
              enabled: form.braveEnabled,
              max_results: parseIntField(form.braveMaxResults, "Brave max results", { min: 1, max: 20 }),
            },
            tavily: {
              enabled: form.tavilyEnabled,
              max_results: parseIntField(form.tavilyMaxResults, "Tavily max results", { min: 1, max: 20 }),
            },
            perplexity: {
              enabled: form.perplexityEnabled,
              max_results: parseIntField(form.perplexityMaxResults, "Perplexity max results", { min: 1, max: 20 }),
            },
            searxng: {
              enabled: form.searxngEnabled,
              base_url: form.searxngBaseUrl,
              max_results: parseIntField(form.searxngMaxResults, "SearXNG max results", { min: 1, max: 20 }),
            },
          },
          mcp: {
            enabled: form.mcpEnabled,
            discovery: {
              enabled: form.mcpDiscoveryEnabled,
              max_search_results: parseIntField(form.mcpDiscoveryMaxResults, "MCP max results", { min: 1, max: 20 }),
            },
          },
          skills: {
            enabled: form.skillsEnabled,
            max_concurrent_searches: parseIntField(form.skillsMaxConcurrentSearches, "Max concurrent searches", { min: 1, max: 10 }),
          },
          media_cleanup: {
            enabled: form.mediaCleanupEnabled,
            max_age_minutes: parseIntField(form.mediaCleanupMaxAgeMinutes, "Max age", { min: 1 }),
            interval_minutes: parseIntField(form.mediaCleanupIntervalMinutes, "Interval", { min: 1 }),
          },
          result_persistence: {
            enabled: form.resultPersistenceEnabled,
            default_max_chars: parseIntField(form.resultPersistenceDefaultMaxChars, "Default max chars", { min: 1000 }),
            per_turn_budget_chars: parseIntField(form.resultPersistencePerTurnBudget, "Per turn budget", { min: 1000 }),
            preview_size_bytes: parseIntField(form.resultPersistencePreviewSize, "Preview size", { min: 100 }),
          },
          wallet: {
            enabled: form.walletEnabled,
            chain: form.walletChain,
            max_send_amount: parseIntField(form.walletMaxSendAmount, "Max send", { min: 0 }),
            max_trade_amount: parseIntField(form.walletMaxTradeAmount, "Max trade", { min: 0 }),
            max_pay_amount: parseIntField(form.walletMaxPayAmount, "Max pay", { min: 0 }),
          },
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
      <PageHeader title="Tools Configuration" />
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

              {/* ── File Tools ── */}
              <ConfigSectionCard title="File Tools" description="File system access tools.">
                <SwitchCardField label="Read File" hint="Read file contents." layout="setting-row" checked={form.readFile} onCheckedChange={(v) => update("readFile", v)} />
                {form.readFile && (
                  <Field label="Max Read Size (bytes)" hint="Maximum file size that can be read." layout="setting-row">
                    <Input type="number" min={1024} value={form.maxReadFileSize} onChange={(e) => update("maxReadFileSize", e.target.value)} />
                  </Field>
                )}
                <SwitchCardField label="Write File" hint="Create or overwrite files." layout="setting-row" checked={form.writeFile} onCheckedChange={(v) => update("writeFile", v)} />
                <SwitchCardField label="Edit File" hint="Search-and-replace edits." layout="setting-row" checked={form.editFile} onCheckedChange={(v) => update("editFile", v)} />
                <SwitchCardField label="Append File" hint="Append content to files." layout="setting-row" checked={form.appendFile} onCheckedChange={(v) => update("appendFile", v)} />
                <SwitchCardField label="List Directory" hint="List files and directories." layout="setting-row" checked={form.listDir} onCheckedChange={(v) => update("listDir", v)} />
                <SwitchCardField label="Send File" hint="Send files to chat." layout="setting-row" checked={form.sendFile} onCheckedChange={(v) => update("sendFile", v)} />
              </ConfigSectionCard>

              {/* ── Agent Tools ── */}
              <ConfigSectionCard title="Agent Tools" description="Subagent spawning and messaging.">
                <SwitchCardField label="Subagent" hint="Synchronous subagent task execution." layout="setting-row" checked={form.subagent} onCheckedChange={(v) => update("subagent", v)} />
                <SwitchCardField label="Spawn" hint="Async background subagent spawning." layout="setting-row" checked={form.spawn} onCheckedChange={(v) => update("spawn", v)} />
                <SwitchCardField label="Spawn Status" hint="Check status of spawned subagents." layout="setting-row" checked={form.spawnStatus} onCheckedChange={(v) => update("spawnStatus", v)} />
                <SwitchCardField label="Message" hint="Send messages to chat channels." layout="setting-row" checked={form.message} onCheckedChange={(v) => update("message", v)} />
              </ConfigSectionCard>

              {/* ── Web Search ── */}
              <ConfigSectionCard title="Web Search" description="Web search providers and fetch settings.">
                <SwitchCardField label="Web Search" hint="Enable web search tools." layout="setting-row" checked={form.webEnabled} onCheckedChange={(v) => update("webEnabled", v)} />
                {form.webEnabled && (
                  <>
                    <SwitchCardField label="Prefer Native" hint="Use provider's native search if available." layout="setting-row" checked={form.webPreferNative} onCheckedChange={(v) => update("webPreferNative", v)} />
                    <SwitchCardField label="DuckDuckGo" hint="DuckDuckGo search provider." layout="setting-row" checked={form.duckduckgoEnabled} onCheckedChange={(v) => update("duckduckgoEnabled", v)} />
                    {form.duckduckgoEnabled && (
                      <Field label="DuckDuckGo Max Results" layout="setting-row">
                        <Input type="number" min={1} max={20} value={form.duckduckgoMaxResults} onChange={(e) => update("duckduckgoMaxResults", e.target.value)} />
                      </Field>
                    )}
                    <SwitchCardField label="Brave Search" hint="Brave search provider (requires API key)." layout="setting-row" checked={form.braveEnabled} onCheckedChange={(v) => update("braveEnabled", v)} />
                    {form.braveEnabled && (
                      <Field label="Brave Max Results" layout="setting-row">
                        <Input type="number" min={1} max={20} value={form.braveMaxResults} onChange={(e) => update("braveMaxResults", e.target.value)} />
                      </Field>
                    )}
                    <SwitchCardField label="Tavily" hint="Tavily search provider (requires API key)." layout="setting-row" checked={form.tavilyEnabled} onCheckedChange={(v) => update("tavilyEnabled", v)} />
                    <SwitchCardField label="Perplexity" hint="Perplexity search provider (requires API key)." layout="setting-row" checked={form.perplexityEnabled} onCheckedChange={(v) => update("perplexityEnabled", v)} />
                    <SwitchCardField label="SearXNG" hint="Self-hosted SearXNG instance." layout="setting-row" checked={form.searxngEnabled} onCheckedChange={(v) => update("searxngEnabled", v)} />
                    {form.searxngEnabled && (
                      <Field label="SearXNG Base URL" layout="setting-row">
                        <Input value={form.searxngBaseUrl} onChange={(e) => update("searxngBaseUrl", e.target.value)} placeholder="http://localhost:8888" />
                      </Field>
                    )}
                  </>
                )}
                <SwitchCardField label="Web Fetch" hint="Fetch and read web pages." layout="setting-row" checked={form.webFetch} onCheckedChange={(v) => update("webFetch", v)} />
                <Field label="Fetch Limit (bytes)" hint="Maximum size for web page fetches." layout="setting-row">
                  <Input type="number" min={0} value={form.webFetchLimitBytes} onChange={(e) => update("webFetchLimitBytes", e.target.value)} />
                </Field>
              </ConfigSectionCard>

              {/* ── Skills ── */}
              <ConfigSectionCard title="Skills" description="Skill discovery and installation.">
                <SwitchCardField label="Skills System" hint="Enable the skills framework." layout="setting-row" checked={form.skillsEnabled} onCheckedChange={(v) => update("skillsEnabled", v)} />
                {form.skillsEnabled && (
                  <>
                    <SwitchCardField label="Find Skills" hint="Search for skills in registries." layout="setting-row" checked={form.findSkills} onCheckedChange={(v) => update("findSkills", v)} />
                    <SwitchCardField label="Install Skills" hint="Install skills from registries." layout="setting-row" checked={form.installSkill} onCheckedChange={(v) => update("installSkill", v)} />
                    <Field label="Max Concurrent Searches" layout="setting-row">
                      <Input type="number" min={1} max={10} value={form.skillsMaxConcurrentSearches} onChange={(e) => update("skillsMaxConcurrentSearches", e.target.value)} />
                    </Field>
                  </>
                )}
              </ConfigSectionCard>

              {/* ── MCP ── */}
              <ConfigSectionCard title="MCP" description="Model Context Protocol tool servers.">
                <SwitchCardField label="MCP" hint="Enable MCP tool server connections." layout="setting-row" checked={form.mcpEnabled} onCheckedChange={(v) => update("mcpEnabled", v)} />
                {form.mcpEnabled && (
                  <>
                    <SwitchCardField label="Discovery" hint="Auto-discover MCP servers." layout="setting-row" checked={form.mcpDiscoveryEnabled} onCheckedChange={(v) => update("mcpDiscoveryEnabled", v)} />
                    <Field label="Discovery Max Results" layout="setting-row">
                      <Input type="number" min={1} max={20} value={form.mcpDiscoveryMaxResults} onChange={(e) => update("mcpDiscoveryMaxResults", e.target.value)} />
                    </Field>
                  </>
                )}
              </ConfigSectionCard>

              {/* ── Result Persistence ── */}
              <ConfigSectionCard title="Result Persistence" description="How tool results are stored and truncated.">
                <SwitchCardField label="Enabled" hint="Persist large tool results to disk." layout="setting-row" checked={form.resultPersistenceEnabled} onCheckedChange={(v) => update("resultPersistenceEnabled", v)} />
                {form.resultPersistenceEnabled && (
                  <>
                    <Field label="Default Max Chars" hint="Max characters per result." layout="setting-row">
                      <Input type="number" min={1000} value={form.resultPersistenceDefaultMaxChars} onChange={(e) => update("resultPersistenceDefaultMaxChars", e.target.value)} />
                    </Field>
                    <Field label="Per Turn Budget" hint="Total character budget per turn." layout="setting-row">
                      <Input type="number" min={1000} value={form.resultPersistencePerTurnBudget} onChange={(e) => update("resultPersistencePerTurnBudget", e.target.value)} />
                    </Field>
                    <Field label="Preview Size (bytes)" hint="Preview size for truncated results." layout="setting-row">
                      <Input type="number" min={100} value={form.resultPersistencePreviewSize} onChange={(e) => update("resultPersistencePreviewSize", e.target.value)} />
                    </Field>
                  </>
                )}
              </ConfigSectionCard>

              {/* ── Media & Security ── */}
              <ConfigSectionCard title="Media & Security" description="Media cleanup and data filtering.">
                <SwitchCardField label="Media Cleanup" hint="Auto-delete temporary media files." layout="setting-row" checked={form.mediaCleanupEnabled} onCheckedChange={(v) => update("mediaCleanupEnabled", v)} />
                {form.mediaCleanupEnabled && (
                  <>
                    <Field label="Max Age (minutes)" layout="setting-row">
                      <Input type="number" min={1} value={form.mediaCleanupMaxAgeMinutes} onChange={(e) => update("mediaCleanupMaxAgeMinutes", e.target.value)} />
                    </Field>
                    <Field label="Cleanup Interval (minutes)" layout="setting-row">
                      <Input type="number" min={1} value={form.mediaCleanupIntervalMinutes} onChange={(e) => update("mediaCleanupIntervalMinutes", e.target.value)} />
                    </Field>
                  </>
                )}
                <SwitchCardField label="Filter Sensitive Data" hint="Redact API keys and tokens from tool output." layout="setting-row" checked={form.filterSensitiveData} onCheckedChange={(v) => update("filterSensitiveData", v)} />
                {form.filterSensitiveData && (
                  <Field label="Filter Min Length" hint="Minimum string length to consider for filtering." layout="setting-row">
                    <Input type="number" min={0} value={form.filterMinLength} onChange={(e) => update("filterMinLength", e.target.value)} />
                  </Field>
                )}
              </ConfigSectionCard>

              {/* ── Wallet ── */}
              <ConfigSectionCard title="Wallet" description="Crypto wallet and payment tools.">
                <SwitchCardField label="Wallet" hint="Enable crypto wallet tools." layout="setting-row" checked={form.walletEnabled} onCheckedChange={(v) => update("walletEnabled", v)} />
                {form.walletEnabled && (
                  <>
                    <Field label="Chain" hint="Blockchain network." layout="setting-row">
                      <Input value={form.walletChain} onChange={(e) => update("walletChain", e.target.value)} />
                    </Field>
                    <Field label="Max Send Amount" layout="setting-row">
                      <Input type="number" min={0} value={form.walletMaxSendAmount} onChange={(e) => update("walletMaxSendAmount", e.target.value)} />
                    </Field>
                    <Field label="Max Trade Amount" layout="setting-row">
                      <Input type="number" min={0} value={form.walletMaxTradeAmount} onChange={(e) => update("walletMaxTradeAmount", e.target.value)} />
                    </Field>
                    <Field label="Max Pay Amount" layout="setting-row">
                      <Input type="number" min={0} value={form.walletMaxPayAmount} onChange={(e) => update("walletMaxPayAmount", e.target.value)} />
                    </Field>
                  </>
                )}
              </ConfigSectionCard>

              {/* ── Hardware ── */}
              <ConfigSectionCard title="Hardware" description="Hardware interface tools.">
                <SwitchCardField label="I2C" hint="I2C bus communication." layout="setting-row" checked={form.i2c} onCheckedChange={(v) => update("i2c", v)} />
                <SwitchCardField label="SPI" hint="SPI bus communication." layout="setting-row" checked={form.spi} onCheckedChange={(v) => update("spi", v)} />
              </ConfigSectionCard>

              {/* ── Save/Reset ── */}
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
