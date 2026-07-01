import { IconHistory, IconTrash } from "@tabler/icons-react"
import dayjs from "dayjs"
import { useCallback, useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { useSessionHistory } from "@/hooks/use-session-history"
import { cn } from "@/lib/utils"

interface MobileSessionHistoryProps {
  activeSessionId: string
  onSwitchSession: (sessionId: string) => void
  onNewChat: () => void
}

export function MobileSessionHistory({
  activeSessionId,
  onSwitchSession,
  onNewChat,
}: MobileSessionHistoryProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [initialLoading, setInitialLoading] = useState(false)

  const {
    sessions,
    hasMore,
    loadError,
    loadErrorMessage,
    observerRef,
    loadSessions,
    handleDeleteSession,
  } = useSessionHistory({
    activeSessionId,
    onDeletedActiveSession: onNewChat,
  })

  const handleOpenChange = useCallback(
    (open: boolean) => {
      setOpen(open)
      if (open) {
        setInitialLoading(true)
        loadSessions(true).finally(() => setInitialLoading(false))
      }
    },
    [loadSessions],
  )

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>
        <Button
          type="button"
          variant="secondary"
          size="icon"
          className="h-9 w-9 shrink-0"
          aria-label={t("chat.history")}
          title={t("chat.history")}
        >
          <IconHistory className="size-5" />
        </Button>
      </SheetTrigger>
      <SheetContent
        side="bottom"
        className="flex max-h-[70vh] flex-col pb-[calc(1rem+env(safe-area-inset-bottom))]"
      >
        <SheetHeader>
          <SheetTitle>{t("chat.history")}</SheetTitle>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto -mx-4 px-4">
          {/* Loading skeleton */}
          {initialLoading && (
            <div className="space-y-3 py-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full rounded-lg" />
              ))}
            </div>
          )}

          {/* Error banner */}
          {loadError && !initialLoading && (
            <div className="text-destructive bg-destructive/10 rounded-lg px-4 py-3 text-sm">
              {loadErrorMessage}
            </div>
          )}

          {/* Empty state */}
          {!initialLoading && !loadError && sessions.length === 0 && (
            <div className="text-muted-foreground py-8 text-center text-sm">
              {t("chat.noHistory")}
            </div>
          )}

          {/* Session list */}
          {sessions.map((session) => (
            <div
              key={session.id}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-3",
                session.id === activeSessionId
                  ? "bg-accent"
                  : "hover:bg-muted/50",
              )}
            >
              <button
                type="button"
                className="min-w-0 flex-1 text-left"
                onClick={() => {
                  onSwitchSession(session.id)
                  setOpen(false)
                }}
              >
                <div className="truncate text-sm font-medium">
                  {session.title || t("chat.newChat")}
                </div>
                <div className="text-muted-foreground text-xs">
                  {t("chat.messagesCount", { count: session.message_count })}
                  {" · "}
                  {dayjs(session.updated).fromNow()}
                </div>
              </button>
              <Button
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive h-8 w-8 shrink-0"
                aria-label={t("chat.deleteSession")}
                onClick={(e) => {
                  e.stopPropagation()
                  handleDeleteSession(session.id)
                }}
              >
                <IconTrash className="size-4" />
              </Button>
            </div>
          ))}

          {/* Infinite scroll sentinel */}
          {hasMore && sessions.length > 0 && (
            <div
              ref={observerRef}
              className="text-muted-foreground py-3 text-center text-xs"
            >
              {t("chat.loadingMore")}
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
