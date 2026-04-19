import {
  IconCheck,
  IconChevronDown,
  IconChevronRight,
  IconLoader2,
  IconRefresh,
  IconTool,
  IconX,
} from "@tabler/icons-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"

export type ToolCallStatus = "pending" | "running" | "completed" | "error"

export interface ToolCall {
  id: string
  tool: string
  status: ToolCallStatus
  input?: Record<string, unknown>
  output?: string
  errorMessage?: string
}

interface ToolCallCardProps {
  toolCall: ToolCall
}

function StatusIcon({ status }: { status: ToolCallStatus }) {
  switch (status) {
    case "pending":
      return (
        <IconLoader2 className="text-muted-foreground size-3.5 animate-spin opacity-60" />
      )
    case "running":
      return <IconLoader2 className="size-3.5 animate-spin text-violet-400" />
    case "completed":
      return <IconCheck className="size-3.5 text-green-500" />
    case "error":
      return <IconX className="size-3.5 text-red-500" />
  }
}

function StatusLabel({ status }: { status: ToolCallStatus }) {
  const { t } = useTranslation()
  const label = t(`chat.toolCall.status.${status}`)
  const colorClass =
    status === "completed"
      ? "text-green-600 dark:text-green-400"
      : status === "error"
        ? "text-red-500"
        : status === "running"
          ? "text-violet-500"
          : "text-muted-foreground"
  return <span className={`text-xs ${colorClass}`}>{label}</span>
}

export function ToolCallCard({ toolCall }: ToolCallCardProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)

  if (toolCall.tool === "end_turn") {
    const continuation = toolCall.input?.continuation as string | undefined
    const intent = toolCall.input?.intent as string | undefined
    const reason = toolCall.input?.reason as string | undefined
    const label = intent
      ? `↻ Agent continued — ${continuation ?? "continue_now"}: "${intent}"`
      : `↻ Agent continued autonomously`
    const hasDetails = !!intent || !!reason

    return (
      <Collapsible
        open={isOpen}
        onOpenChange={setIsOpen}
        className="border-border/40 rounded-lg border border-dashed"
      >
        <CollapsibleTrigger
          className="flex w-full cursor-pointer items-center gap-2 px-3 py-1.5 text-left select-none"
          disabled={!hasDetails}
        >
          <IconRefresh className="text-muted-foreground size-3.5 shrink-0" />
          <span className="text-muted-foreground min-w-0 flex-1 truncate text-xs italic">
            {label}
          </span>
          {hasDetails && (
            <span className="text-muted-foreground ml-1 shrink-0">
              {isOpen ? (
                <IconChevronDown className="size-3.5" />
              ) : (
                <IconChevronRight className="size-3.5" />
              )}
            </span>
          )}
        </CollapsibleTrigger>
        {hasDetails && (
          <CollapsibleContent>
            <div className="border-border/40 border-t px-3 pb-2 pt-1">
              {intent && (
                <p className="text-muted-foreground text-xs">
                  <span className="font-medium">intent:</span> {intent}
                </p>
              )}
              {reason && (
                <p className="text-muted-foreground text-xs">
                  <span className="font-medium">reason:</span> {reason}
                </p>
              )}
            </div>
          </CollapsibleContent>
        )}
      </Collapsible>
    )
  }

  const hasDetails =
    (toolCall.input && Object.keys(toolCall.input).length > 0) ||
    toolCall.output !== undefined ||
    toolCall.errorMessage !== undefined

  return (
    <Collapsible
      open={isOpen}
      onOpenChange={setIsOpen}
      className="bg-muted/40 border-border/60 rounded-lg border"
    >
      <CollapsibleTrigger
        className="flex w-full cursor-pointer items-center gap-2 px-3 py-2 text-left select-none"
        disabled={!hasDetails}
      >
        <IconTool className="text-muted-foreground size-3.5 shrink-0" />
        <span className="min-w-0 flex-1 truncate font-mono text-xs font-medium">
          {toolCall.tool}
        </span>
        <StatusIcon status={toolCall.status} />
        <StatusLabel status={toolCall.status} />
        {hasDetails && (
          <span className="text-muted-foreground ml-1 shrink-0">
            {isOpen ? (
              <IconChevronDown className="size-3.5" />
            ) : (
              <IconChevronRight className="size-3.5" />
            )}
          </span>
        )}
      </CollapsibleTrigger>

      {hasDetails && (
        <CollapsibleContent>
          <div className="border-border/40 border-t px-3 pb-3">
            {toolCall.input && Object.keys(toolCall.input).length > 0 && (
              <div className="mt-2">
                <p className="text-muted-foreground mb-1 text-xs font-medium uppercase tracking-wide">
                  {t("chat.toolCall.input")}
                </p>
                <pre className="bg-background/60 border-border/40 overflow-x-auto rounded border p-2 font-mono text-xs leading-relaxed">
                  {JSON.stringify(toolCall.input, null, 2)}
                </pre>
              </div>
            )}

            {toolCall.output !== undefined && (
              <div className="mt-2">
                <p className="text-muted-foreground mb-1 text-xs font-medium uppercase tracking-wide">
                  {t("chat.toolCall.output")}
                </p>
                <pre className="bg-background/60 border-border/40 overflow-x-auto rounded border p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap">
                  {toolCall.output}
                </pre>
              </div>
            )}

            {toolCall.errorMessage !== undefined && (
              <div className="mt-2">
                <p className="mb-1 text-xs font-medium uppercase tracking-wide text-red-500">
                  {t("chat.toolCall.error")}
                </p>
                <pre className="overflow-x-auto rounded border border-red-200 bg-red-50 p-2 font-mono text-xs leading-relaxed whitespace-pre-wrap text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-400">
                  {toolCall.errorMessage}
                </pre>
              </div>
            )}
          </div>
        </CollapsibleContent>
      )}
    </Collapsible>
  )
}
