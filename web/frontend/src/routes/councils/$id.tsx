import { createFileRoute } from "@tanstack/react-router"

import { CouncilTranscript } from "@/components/council/council-transcript"

export const Route = createFileRoute("/councils/$id")({
  component: CouncilRoute,
})

function CouncilRoute() {
  const { id } = Route.useParams()
  return <CouncilTranscript councilId={id} />
}
