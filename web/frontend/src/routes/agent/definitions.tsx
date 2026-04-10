import { createFileRoute } from "@tanstack/react-router"

import { AgentsPage } from "@/components/agents/agents-page"

export const Route = createFileRoute("/agent/definitions")({
  component: AgentsPage,
})
