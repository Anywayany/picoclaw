import {
  IconArrowUp,
  IconFilePlus,
  IconFileText,
  IconPhotoPlus,
  IconX,
} from "@tabler/icons-react"
import {
  type ChangeEvent,
  type KeyboardEvent,
  type RefObject,
  useRef,
} from "react"
import { useTranslation } from "react-i18next"
import TextareaAutosize from "react-textarea-autosize"

import type { ModelInfo } from "@/api/models"
import { ModelSelector } from "@/components/chat/model-selector"
import { Button } from "@/components/ui/button"
import { CHAT_IMAGE_ACCEPT } from "@/features/chat/image-input"
import { cn } from "@/lib/utils"
import type { ChatAttachment } from "@/store/chat"

interface MobileChatComposerProps {
  input: string
  attachments: ChatAttachment[]
  imageInputRef: RefObject<HTMLInputElement | null>
  fileInputRef: RefObject<HTMLInputElement | null>
  placeholder: string
  disabled: boolean
  canSend: boolean
  defaultModelName: string
  apiKeyModels: ModelInfo[]
  oauthModels: ModelInfo[]
  localModels: ModelInfo[]
  hasAvailableModels: boolean
  onSetDefaultModel: (modelName: string) => void
  onInputChange: (value: string) => void
  onImageChange: (event: ChangeEvent<HTMLInputElement>) => void
  onFileChange: (event: ChangeEvent<HTMLInputElement>) => void
  onAddImages: () => void
  onAddFiles: () => void
  onRemoveAttachment: (index: number) => void
  onSend: () => void
}

export function MobileChatComposer({
  input,
  attachments,
  imageInputRef,
  fileInputRef,
  placeholder,
  disabled,
  canSend,
  defaultModelName,
  apiKeyModels,
  oauthModels,
  localModels,
  hasAvailableModels,
  onSetDefaultModel,
  onInputChange,
  onImageChange,
  onFileChange,
  onAddImages,
  onAddFiles,
  onRemoveAttachment,
  onSend,
}: MobileChatComposerProps) {
  const { t } = useTranslation()
  const composingRef = useRef(false)

  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    const nativeEvent = event.nativeEvent as Event & {
      isComposing?: boolean
      keyCode?: number
    }
    if (
      composingRef.current ||
      nativeEvent.isComposing ||
      nativeEvent.keyCode === 229
    ) {
      return
    }
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault()
      onSend()
    }
  }

  return (
    <div className="border-border/60 bg-background shrink-0 border-t px-3 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
      <input
        ref={imageInputRef}
        type="file"
        accept={CHAT_IMAGE_ACCEPT}
        multiple
        className="hidden"
        onChange={onImageChange}
      />
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={onFileChange}
      />

      <div className="mx-auto flex max-w-3xl flex-col gap-2">
        {hasAvailableModels && (
          <div className="flex">
            <ModelSelector
              defaultModelName={defaultModelName}
              apiKeyModels={apiKeyModels}
              oauthModels={oauthModels}
              localModels={localModels}
              onValueChange={onSetDefaultModel}
            />
          </div>
        )}

        {attachments.length > 0 ? (
          <div className="flex gap-2 overflow-x-auto pb-1">
            {attachments.map((attachment, index) => (
              <div
                key={`${attachment.url}-${index}`}
                className="bg-muted relative h-16 w-16 shrink-0 overflow-hidden rounded-md border"
              >
                {attachment.type === "image" ? (
                  <img
                    src={attachment.url}
                    alt={attachment.filename || t("chat.uploadedImage")}
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <div className="flex h-full w-full flex-col items-center justify-center gap-1 p-1.5 text-center">
                    <IconFileText className="text-muted-foreground size-5" />
                    <span className="line-clamp-2 text-[9px] leading-tight">
                      {attachment.filename || t("chat.uploadedFile")}
                    </span>
                  </div>
                )}
                <button
                  type="button"
                  onClick={() => onRemoveAttachment(index)}
                  className="bg-background/90 text-foreground absolute top-1 right-1 inline-flex h-5 w-5 items-center justify-center rounded-full border shadow-sm"
                  aria-label={t("chat.removeAttachment")}
                  title={t("chat.removeAttachment")}
                >
                  <IconX className="size-3" />
                </button>
              </div>
            ))}
          </div>
        ) : null}

        <div className="border-border/70 bg-card flex items-end gap-2 rounded-lg border p-2 shadow-sm">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="text-muted-foreground hover:text-foreground h-9 w-9 shrink-0 rounded-md"
            onClick={onAddImages}
            disabled={disabled}
            aria-label={t("chat.attachImage")}
            title={t("chat.attachImage")}
          >
            <IconPhotoPlus className="size-5" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="text-muted-foreground hover:text-foreground h-9 w-9 shrink-0 rounded-md"
            onClick={onAddFiles}
            disabled={disabled}
            aria-label={t("chat.attachFile")}
            title={t("chat.attachFile")}
          >
            <IconFilePlus className="size-5" />
          </Button>

          <TextareaAutosize
            value={input}
            onChange={(event) => onInputChange(event.target.value)}
            onCompositionStart={() => {
              composingRef.current = true
            }}
            onCompositionEnd={() => {
              composingRef.current = false
            }}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            disabled={disabled}
            minRows={1}
            maxRows={5}
            className={cn(
              "placeholder:text-muted-foreground/60 max-h-36 min-h-9 flex-1 resize-none border-0 bg-transparent px-1 py-2 text-base leading-5 shadow-none outline-none focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-70",
            )}
          />

          <Button
            type="button"
            size="icon"
            className="h-9 w-9 shrink-0 rounded-md"
            onClick={onSend}
            disabled={!canSend}
            aria-label={t("chat.sendMessage")}
            title={t("chat.sendMessage")}
          >
            <IconArrowUp className="size-5" />
          </Button>
        </div>
      </div>
    </div>
  )
}
