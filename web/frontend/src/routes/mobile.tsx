import { createFileRoute } from "@tanstack/react-router"

import { MobileChatPage } from "@/features/mobile/mobile-chat-page"

export const Route = createFileRoute("/mobile")({
  component: MobileChatPage,
})
