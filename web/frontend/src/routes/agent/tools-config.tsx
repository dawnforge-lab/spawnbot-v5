import { createFileRoute } from "@tanstack/react-router"

import { ToolsConfigPage } from "@/components/tools-config/tools-config-page"

export const Route = createFileRoute("/agent/tools-config")({
  component: ToolsConfigPage,
})
