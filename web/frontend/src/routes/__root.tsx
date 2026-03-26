import { Outlet, createRootRoute } from "@tanstack/react-router"
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools"
import { useCallback, useEffect, useState } from "react"

import { getOnboardingStatus } from "@/api/onboarding"
import { AppLayout } from "@/components/app-layout"
import { OnboardingWizard } from "@/components/onboarding/onboarding-wizard"
import { initializeChatStore } from "@/features/chat/controller"

const RootLayout = () => {
  const [onboardingChecked, setOnboardingChecked] = useState(false)
  const [onboardingCompleted, setOnboardingCompleted] = useState(false)

  useEffect(() => {
    getOnboardingStatus()
      .then((status) => {
        setOnboardingCompleted(status.completed)
        setOnboardingChecked(true)
      })
      .catch(() => {
        // If the API is unreachable, assume onboarding is complete
        // (e.g. config may have been created manually).
        setOnboardingCompleted(true)
        setOnboardingChecked(true)
      })
  }, [])

  useEffect(() => {
    if (onboardingCompleted) {
      initializeChatStore()
    }
  }, [onboardingCompleted])

  const handleOnboardingComplete = useCallback(() => {
    setOnboardingCompleted(true)
  }, [])

  // Wait for the status check before rendering anything.
  if (!onboardingChecked) {
    return null
  }

  // Show the onboarding wizard if setup has not been completed.
  if (!onboardingCompleted) {
    return <OnboardingWizard onComplete={handleOnboardingComplete} />
  }

  return (
    <AppLayout>
      <Outlet />
      <TanStackRouterDevtools />
    </AppLayout>
  )
}

export const Route = createRootRoute({ component: RootLayout })
