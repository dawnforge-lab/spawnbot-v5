import { createFileRoute } from "@tanstack/react-router"

import { SystemSettingsPage } from "@/components/system-settings/system-settings-page"

export const Route = createFileRoute("/system")({
  component: SystemSettingsPage,
})
