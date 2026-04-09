import { createFileRoute } from "@tanstack/react-router"

import { CouncilList } from "@/components/council/council-list"

export const Route = createFileRoute("/councils/")({
  component: CouncilList,
})
