// API client for onboarding wizard endpoints.

export interface OnboardingStatus {
  completed: boolean
}

export interface ValidateKeyRequest {
  provider: string
  api_key: string
}

export interface ValidateKeyResponse {
  valid: boolean
  error?: string
}

export interface OnboardingCompleteRequest {
  provider: string
  api_key: string
  user_name: string
  approval_mode: string
  telegram_enabled: boolean
  telegram_token: string
  embedding_provider: string
  embedding_api_key: string
  custom_base_url?: string
}

export interface OnboardingCompleteResponse {
  success: boolean
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, options)
  if (!res.ok) {
    let message = `API error: ${res.status} ${res.statusText}`
    try {
      const body = (await res.json()) as { error?: string }
      if (typeof body.error === "string" && body.error.trim() !== "") {
        message = body.error
      }
    } catch {
      // Keep fallback error message when response body is not JSON.
    }
    throw new Error(message)
  }
  return res.json() as Promise<T>
}

export async function getOnboardingStatus(): Promise<OnboardingStatus> {
  return request<OnboardingStatus>("/api/onboarding/status")
}

export async function validateKey(
  payload: ValidateKeyRequest,
): Promise<ValidateKeyResponse> {
  return request<ValidateKeyResponse>("/api/onboarding/validate-key", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}

export async function completeOnboarding(
  payload: OnboardingCompleteRequest,
): Promise<OnboardingCompleteResponse> {
  return request<OnboardingCompleteResponse>("/api/onboarding/complete", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  })
}
