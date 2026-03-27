import { useCallback, useEffect, useState } from "react"

import {
  type OnboardingCompleteRequest,
  type ValidateKeyResponse,
  completeOnboarding,
  getOnboardingStatus,
  validateKey,
} from "@/api/onboarding"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type WizardStep =
  | "loading"
  | "provider"
  | "api-key"
  | "user-name"
  | "approval-mode"
  | "telegram"
  | "embeddings"
  | "review"
  | "submitting"
  | "done"

interface WizardState {
  provider: string
  apiKey: string
  customBaseUrl: string
  userName: string
  approvalMode: string
  telegramEnabled: boolean
  telegramToken: string
  embeddingProvider: string
  embeddingApiKey: string
}

const initialState: WizardState = {
  provider: "openrouter",
  apiKey: "",
  customBaseUrl: "",
  userName: "",
  approvalMode: "yolo",
  telegramEnabled: false,
  telegramToken: "",
  embeddingProvider: "same",
  embeddingApiKey: "",
}

const providerLabels: Record<string, string> = {
  openrouter: "OpenRouter (recommended)",
  anthropic: "Anthropic (Claude)",
  openai: "OpenAI",
  custom: "Custom OpenAI-compatible endpoint",
}

const approvalModeLabels: Record<string, string> = {
  yolo: "YOLO -- auto-approve all tools",
  approval: "Approval -- ask before dangerous actions",
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function OnboardingWizard({
  onComplete,
}: {
  onComplete: () => void
}) {
  const [step, setStep] = useState<WizardStep>("loading")
  const [state, setState] = useState<WizardState>(initialState)
  const [error, setError] = useState<string | null>(null)
  const [validating, setValidating] = useState(false)

  // Check onboarding status on mount
  useEffect(() => {
    getOnboardingStatus()
      .then((status) => {
        if (status.completed) {
          onComplete()
        } else {
          setStep("provider")
        }
      })
      .catch(() => {
        // If we can't reach the API, show the wizard anyway
        setStep("provider")
      })
  }, [onComplete])

  const update = useCallback(
    (patch: Partial<WizardState>) => {
      setState((prev) => ({ ...prev, ...patch }))
      setError(null)
    },
    [],
  )

  const handleValidateKey = useCallback(async () => {
    setValidating(true)
    setError(null)
    try {
      const resp: ValidateKeyResponse = await validateKey({
        provider: state.provider,
        api_key: state.apiKey,
      })
      if (resp.valid) {
        setStep("user-name")
      } else {
        setError(resp.error || "Invalid API key")
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Validation failed")
    } finally {
      setValidating(false)
    }
  }, [state.provider, state.apiKey])

  const handleSubmit = useCallback(async () => {
    setStep("submitting")
    setError(null)
    try {
      const payload: OnboardingCompleteRequest = {
        provider: state.provider,
        api_key: state.apiKey,
        user_name: state.userName || "friend",
        approval_mode: state.approvalMode,
        telegram_enabled: state.telegramEnabled,
        telegram_token: state.telegramToken,
        embedding_provider: state.embeddingProvider,
        embedding_api_key: state.embeddingApiKey,
      }
      if (state.provider === "custom" && state.customBaseUrl) {
        payload.custom_base_url = state.customBaseUrl
      }
      const resp = await completeOnboarding(payload)
      if (resp.success) {
        setStep("done")
        // Brief delay before redirecting so the user sees the success state
        setTimeout(onComplete, 1500)
      } else {
        setError("Onboarding failed")
        setStep("review")
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Onboarding failed")
      setStep("review")
    }
  }, [state, onComplete])

  // -- Loading state --
  if (step === "loading") {
    return (
      <div className="flex h-dvh items-center justify-center">
        <p className="text-muted-foreground">Checking setup status...</p>
      </div>
    )
  }

  // -- Done state --
  if (step === "done") {
    return (
      <div className="flex h-dvh items-center justify-center">
        <Card className="w-full max-w-lg">
          <CardHeader>
            <CardTitle>Setup Complete</CardTitle>
            <CardDescription>
              spawnbot is ready. Redirecting to chat...
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-background p-4">
      <Card className="w-full max-w-lg">
        <CardHeader>
          <CardTitle>Welcome to spawnbot</CardTitle>
          <CardDescription>
            Let&apos;s get you set up in a few steps.
          </CardDescription>
        </CardHeader>

        <CardContent className="space-y-4">
          {error && (
            <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {error}
            </div>
          )}

          {/* Step 1: Provider */}
          {step === "provider" && (
            <div className="space-y-3">
              <Label>Which LLM provider?</Label>
              <Select
                value={state.provider}
                onValueChange={(v) => update({ provider: v })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(providerLabels).map(([value, label]) => (
                    <SelectItem key={value} value={value}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {state.provider === "custom" && (
                <div className="space-y-1.5">
                  <Label>API Base URL</Label>
                  <Input
                    placeholder="http://localhost:8080/v1"
                    value={state.customBaseUrl}
                    onChange={(e) =>
                      update({ customBaseUrl: e.target.value })
                    }
                  />
                </div>
              )}
            </div>
          )}

          {/* Step 2: API Key */}
          {step === "api-key" && (
            <div className="space-y-3">
              <Label>API Key</Label>
              <Input
                type="password"
                placeholder="sk-..."
                value={state.apiKey}
                onChange={(e) => update({ apiKey: e.target.value })}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && state.apiKey) {
                    handleValidateKey()
                  }
                }}
              />
              <p className="text-xs text-muted-foreground">
                Your key is stored locally and never sent to spawnbot servers.
              </p>
            </div>
          )}

          {/* Step 3: User name */}
          {step === "user-name" && (
            <div className="space-y-3">
              <Label>What should I call you?</Label>
              <Input
                placeholder="your name"
                value={state.userName}
                onChange={(e) => update({ userName: e.target.value })}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    setStep("approval-mode")
                  }
                }}
              />
            </div>
          )}

          {/* Step 4: Approval mode */}
          {step === "approval-mode" && (
            <div className="space-y-3">
              <Label>Tool approval mode</Label>
              <Select
                value={state.approvalMode}
                onValueChange={(v) => update({ approvalMode: v })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(approvalModeLabels).map(
                    ([value, label]) => (
                      <SelectItem key={value} value={value}>
                        {label}
                      </SelectItem>
                    ),
                  )}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Step 5: Telegram */}
          {step === "telegram" && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <Label>Connect Telegram?</Label>
                <Switch
                  checked={state.telegramEnabled}
                  onCheckedChange={(v) =>
                    update({ telegramEnabled: v === true })
                  }
                />
              </div>
              {state.telegramEnabled && (
                <div className="space-y-1.5">
                  <Label>Telegram Bot Token</Label>
                  <Input
                    type="password"
                    placeholder="123456:ABC-DEF..."
                    value={state.telegramToken}
                    onChange={(e) =>
                      update({ telegramToken: e.target.value })
                    }
                  />
                </div>
              )}
            </div>
          )}

          {/* Step 6: Embeddings */}
          {step === "embeddings" && (
            <div className="space-y-3">
              <Label>Embedding provider for memory</Label>
              <Select
                value={state.embeddingProvider}
                onValueChange={(v) => update({ embeddingProvider: v })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="same">
                    Same as chat provider
                  </SelectItem>
                  <SelectItem value="gemini">
                    Gemini (free tier)
                  </SelectItem>
                  <SelectItem value="openai">OpenAI</SelectItem>
                </SelectContent>
              </Select>
              {state.embeddingProvider !== "same" && (
                <div className="space-y-1.5">
                  <Label>Embedding API Key</Label>
                  <Input
                    type="password"
                    placeholder="API key for embedding provider"
                    value={state.embeddingApiKey}
                    onChange={(e) =>
                      update({ embeddingApiKey: e.target.value })
                    }
                  />
                </div>
              )}
            </div>
          )}

          {/* Step 7: Review */}
          {step === "review" && (
            <div className="space-y-2 text-sm">
              <p>
                <span className="font-medium">Provider:</span>{" "}
                {providerLabels[state.provider] || state.provider}
              </p>
              <p>
                <span className="font-medium">Name:</span>{" "}
                {state.userName || "friend"}
              </p>
              <p>
                <span className="font-medium">Mode:</span>{" "}
                {approvalModeLabels[state.approvalMode] || state.approvalMode}
              </p>
              {state.telegramEnabled && (
                <p>
                  <span className="font-medium">Telegram:</span> enabled
                </p>
              )}
              <p>
                <span className="font-medium">Embeddings:</span>{" "}
                {state.embeddingProvider === "same"
                  ? "same as chat"
                  : state.embeddingProvider}
              </p>
            </div>
          )}

          {/* Submitting state */}
          {step === "submitting" && (
            <p className="text-muted-foreground">
              Setting up your configuration...
            </p>
          )}
        </CardContent>

        <CardFooter className="flex justify-between">
          {/* Back button */}
          {step !== "provider" &&
            step !== "submitting" &&
            step !== "done" && (
              <Button
                variant="outline"
                onClick={() => {
                  setError(null)
                  const steps: WizardStep[] = [
                    "provider",
                    "api-key",
                    "user-name",
                    "approval-mode",
                    "telegram",
                    "embeddings",
                    "review",
                  ]
                  const idx = steps.indexOf(step)
                  if (idx > 0) {
                    setStep(steps[idx - 1])
                  }
                }}
              >
                Back
              </Button>
            )}

          {/* Spacer when no back button */}
          {(step === "provider" || step === "submitting") && <div />}

          {/* Forward / action button */}
          {step === "provider" && (
            <Button onClick={() => setStep("api-key")}>Next</Button>
          )}
          {step === "api-key" && (
            <Button
              onClick={handleValidateKey}
              disabled={!state.apiKey || validating}
            >
              {validating ? "Validating..." : "Next"}
            </Button>
          )}
          {step === "user-name" && (
            <Button onClick={() => setStep("approval-mode")}>Next</Button>
          )}
          {step === "approval-mode" && (
            <Button onClick={() => setStep("telegram")}>Next</Button>
          )}
          {step === "telegram" && (
            <Button onClick={() => setStep("embeddings")}>Next</Button>
          )}
          {step === "embeddings" && (
            <Button onClick={() => setStep("review")}>Review</Button>
          )}
          {step === "review" && (
            <Button onClick={handleSubmit}>Complete Setup</Button>
          )}
        </CardFooter>
      </Card>
    </div>
  )
}
