import { IconPlus } from "@tabler/icons-react"
import { useAtomValue } from "jotai"
import { type ChangeEvent, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { MobileSessionHistory } from "@/components/chat/mobile-session-history"
import { Button } from "@/components/ui/button"
import {
  buildChatFileAttachments,
  buildChatImageAttachments,
} from "@/features/chat/image-input"
import { MobileChatComposer } from "@/features/mobile/mobile-chat-composer"
import { MobileMessageList } from "@/features/mobile/mobile-message-list"
import { useMobileGatewayPolling } from "@/features/mobile/use-mobile-gateway-polling"
import { useChatModels } from "@/hooks/use-chat-models"
import { usePicoChat } from "@/hooks/use-pico-chat"
import { cn } from "@/lib/utils"
import type { ChatAttachment, ConnectionState } from "@/store/chat"
import { type GatewayState, gatewayAtom } from "@/store/gateway"

function getMobileDisabledKey({
  gatewayStatus,
  connectionState,
}: {
  gatewayStatus: GatewayState
  connectionState: ConnectionState
}): string | null {
  if (gatewayStatus === "running") {
    if (connectionState === "connecting") {
      return "websocketConnecting"
    }
    if (connectionState === "error") {
      return "websocketError"
    }
    if (connectionState === "disconnected") {
      return "websocketDisconnected"
    }
    return null
  }

  if (gatewayStatus === "unknown") {
    return "gatewayUnknown"
  }
  if (gatewayStatus === "starting") {
    return "gatewayStarting"
  }
  if (gatewayStatus === "restarting") {
    return "gatewayRestarting"
  }
  if (gatewayStatus === "stopping") {
    return "gatewayStopping"
  }
  if (gatewayStatus === "error") {
    return "gatewayError"
  }
  return "gatewayStopped"
}

function getConnectionLabelKey(connectionState: ConnectionState) {
  switch (connectionState) {
    case "connected":
      return "mobile.connection.connected"
    case "connecting":
      return "mobile.connection.connecting"
    case "error":
      return "mobile.connection.error"
    case "disconnected":
    default:
      return "mobile.connection.disconnected"
  }
}

export function MobileChatPage() {
  const { t } = useTranslation()
  const imageInputRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [input, setInput] = useState("")
  const [attachments, setAttachments] = useState<ChatAttachment[]>([])
  const { status: gatewayStatus } = useAtomValue(gatewayAtom)
  const {
    messages,
    connectionState,
    isTyping,
    sendMessage,
    newChat,
    activeSessionId,
    switchSession,
  } = usePicoChat()

  useMobileGatewayPolling()

  const {
    defaultModelName,
    hasAvailableModels,
    apiKeyModels,
    oauthModels,
    localModels,
    handleSetDefault,
  } = useChatModels({ isConnected: gatewayStatus === "running" })

  const disabledKey = getMobileDisabledKey({
    gatewayStatus,
    connectionState,
  })
  const disabled = disabledKey !== null
  const placeholder =
    disabledKey === null
      ? t("mobile.placeholder")
      : t(`mobile.disabled.${disabledKey}`)
  const canSend =
    !disabled && (input.trim().length > 0 || attachments.length > 0)

  const statusText = useMemo(() => {
    if (gatewayStatus !== "running") {
      return t(`mobile.gateway.${gatewayStatus}`)
    }
    return t(getConnectionLabelKey(connectionState))
  }, [connectionState, gatewayStatus, t])

  const handleAddImages = () => {
    if (disabled) {
      return
    }
    imageInputRef.current?.click()
  }

  const handleAddFiles = () => {
    if (disabled) {
      return
    }
    fileInputRef.current?.click()
  }

  const handleImageSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ""
    if (disabled || files.length === 0) {
      return
    }

    const nextAttachments = await buildChatImageAttachments(files, t)
    if (nextAttachments.length > 0) {
      setAttachments((prev) => [...prev, ...nextAttachments])
    }
  }

  const handleFileSelection = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ""
    if (disabled || files.length === 0) {
      return
    }

    const nextAttachments = await buildChatFileAttachments(files, t)
    if (nextAttachments.length > 0) {
      setAttachments((prev) => [...prev, ...nextAttachments])
    }
  }

  const handleSend = () => {
    if (!canSend) {
      return
    }
    if (sendMessage({ content: input, attachments })) {
      setInput("")
      setAttachments([])
    }
  }

  const handleNewChat = () => {
    setInput("")
    setAttachments([])
    void newChat()
  }

  return (
    <div className="bg-background flex h-full min-h-0 flex-col">
      <header className="border-border/60 bg-background/95 supports-[backdrop-filter]:bg-background/80 grid h-12 shrink-0 grid-cols-[36px_1fr_36px] items-center border-b px-3 backdrop-blur">
        <MobileSessionHistory
          activeSessionId={activeSessionId}
          onSwitchSession={switchSession}
          onNewChat={newChat}
        />
        <div className="min-w-0 text-center">
          <div className="truncate text-sm font-semibold">DiAgent</div>
          <div
            className={cn(
              "truncate text-xs",
              gatewayStatus === "running" && connectionState === "connected"
                ? "text-emerald-600 dark:text-emerald-400"
                : "text-muted-foreground",
            )}
          >
            {statusText}
          </div>
        </div>

        <Button
          type="button"
          variant="secondary"
          size="icon"
          className="h-9 w-9 shrink-0 justify-self-end"
          onClick={handleNewChat}
          aria-label={t("chat.newChat")}
          title={t("chat.newChat")}
        >
          <IconPlus className="size-5" />
        </Button>
      </header>

      {disabledKey && gatewayStatus !== "running" ? (
        <div className="border-border/60 bg-muted/45 text-muted-foreground border-b px-3 py-2 text-sm">
          {t("mobile.gatewayHint")}
        </div>
      ) : null}

      <MobileMessageList messages={messages} isTyping={isTyping} />

      <MobileChatComposer
        input={input}
        attachments={attachments}
        imageInputRef={imageInputRef}
        fileInputRef={fileInputRef}
        placeholder={placeholder}
        disabled={disabled}
        canSend={canSend}
        defaultModelName={defaultModelName}
        apiKeyModels={apiKeyModels}
        oauthModels={oauthModels}
        localModels={localModels}
        hasAvailableModels={hasAvailableModels}
        onSetDefaultModel={handleSetDefault}
        onInputChange={setInput}
        onImageChange={handleImageSelection}
        onFileChange={handleFileSelection}
        onAddImages={handleAddImages}
        onAddFiles={handleAddFiles}
        onRemoveAttachment={(index) =>
          setAttachments((prev) =>
            prev.filter((_, itemIndex) => itemIndex !== index),
          )
        }
        onSend={handleSend}
      />
    </div>
  )
}
