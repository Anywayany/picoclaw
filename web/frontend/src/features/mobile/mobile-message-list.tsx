import { useAtomValue } from "jotai"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { AssistantMessage } from "@/components/chat/assistant-message"
import { TypingIndicator } from "@/components/chat/typing-indicator"
import { UserMessage } from "@/components/chat/user-message"
import { cn } from "@/lib/utils"
import {
  assistantDetailVisibilityAtom,
  shouldShowAssistantMessage,
  type ChatMessage,
} from "@/store/chat"

interface MobileMessageListProps {
  messages: ChatMessage[]
  isTyping: boolean
}

export function MobileMessageList({
  messages,
  isTyping,
}: MobileMessageListProps) {
  const { t } = useTranslation()
  const scrollRef = useRef<HTMLDivElement>(null)
  const [isAtBottom, setIsAtBottom] = useState(true)
  const assistantDetailVisibility = useAtomValue(assistantDetailVisibilityAtom)

  const handleScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const element = event.currentTarget
    const { clientHeight, scrollHeight, scrollTop } = element
    setIsAtBottom(scrollHeight - scrollTop <= clientHeight + 24)
  }

  useEffect(() => {
    if (!scrollRef.current || !isAtBottom) {
      return
    }
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [messages, isTyping, isAtBottom])

  return (
    <div
      ref={scrollRef}
      onScroll={handleScroll}
      className="min-h-0 flex-1 overflow-y-auto px-3 py-4 [scrollbar-gutter:stable]"
    >
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 pb-4">
        {messages.length === 0 && !isTyping ? (
          <div className="flex min-h-[45dvh] items-center justify-center">
            <div className="text-center">
              <div className="text-foreground text-lg font-semibold">
                {t("mobile.emptyTitle")}
              </div>
              <div className="text-muted-foreground mt-2 text-sm">
                {t("mobile.emptyDescription")}
              </div>
            </div>
          </div>
        ) : null}

        {messages.map((message) => {
          if (
            message.role === "assistant" &&
            !shouldShowAssistantMessage(assistantDetailVisibility, message.kind)
          ) {
            return null
          }

          return (
            <div
              key={message.id}
              className={cn(
                "flex w-full",
                message.role === "user" ? "justify-end" : "justify-start",
              )}
            >
              <div
                className={cn(
                  "w-full",
                  message.role === "user" ? "max-w-[88%]" : "max-w-full",
                )}
              >
                {message.role === "assistant" ? (
                  <AssistantMessage
                    content={message.content}
                    attachments={message.attachments}
                    kind={message.kind}
                    modelName={message.modelName}
                    toolCalls={message.toolCalls}
                    timestamp={message.timestamp}
                  />
                ) : (
                  <UserMessage
                    content={message.content}
                    attachments={message.attachments}
                    timestamp={message.timestamp}
                  />
                )}
              </div>
            </div>
          )
        })}

        {isTyping ? <TypingIndicator /> : null}
      </div>
    </div>
  )
}
