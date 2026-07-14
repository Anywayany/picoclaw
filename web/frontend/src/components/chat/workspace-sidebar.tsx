import {
  IconAlertCircle,
  IconChevronRight,
  IconFile,
  IconFolder,
  IconFolderOpen,
  IconLink,
  IconLoader2,
  IconRefresh,
  IconX,
} from "@tabler/icons-react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { type WorkspaceEntry, getWorkspaceDirectory } from "@/api/workspace"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { useIsMobile } from "@/hooks/use-mobile"
import { cn } from "@/lib/utils"

const workspaceQueryKey = ["workspace", "directory"] as const
const workspaceSidebarWidthKey = "picoclaw.workspaceSidebarWidth"
const defaultWorkspaceSidebarWidth = 320
const minWorkspaceSidebarWidth = 260
const minChatWidth = 320

function loadWorkspaceSidebarWidth(): number {
  if (typeof globalThis.localStorage === "undefined") {
    return defaultWorkspaceSidebarWidth
  }
  const storedWidth = Number(
    globalThis.localStorage.getItem(workspaceSidebarWidthKey),
  )
  return Number.isFinite(storedWidth) && storedWidth >= minWorkspaceSidebarWidth
    ? storedWidth
    : defaultWorkspaceSidebarWidth
}

function formatFileSize(size?: number): string | null {
  if (!size) return null
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  if (size < 1024 * 1024 * 1024) {
    return `${(size / (1024 * 1024)).toFixed(1)} MB`
  }
  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`
}

function EntryIcon({
  entry,
  expanded,
}: {
  entry: WorkspaceEntry
  expanded?: boolean
}) {
  if (entry.type === "directory") {
    return expanded ? (
      <IconFolderOpen className="size-4 shrink-0 text-amber-500" />
    ) : (
      <IconFolder className="size-4 shrink-0 text-amber-500" />
    )
  }
  if (entry.type === "symlink") {
    return <IconLink className="text-muted-foreground size-4 shrink-0" />
  }
  return <IconFile className="text-muted-foreground size-4 shrink-0" />
}

function TreeError({ depth }: { depth: number }) {
  const { t } = useTranslation()
  return (
    <div
      className="text-destructive flex items-center gap-2 py-2 pr-2 text-xs"
      style={{ paddingLeft: depth * 12 + 12 }}
      role="alert"
    >
      <IconAlertCircle className="size-3.5 shrink-0" />
      <span>{t("chat.workspace.loadFailed")}</span>
    </div>
  )
}

function DirectoryEntry({
  entry,
  depth,
}: {
  entry: WorkspaceEntry
  depth: number
}) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const directoryQuery = useQuery({
    queryKey: [...workspaceQueryKey, entry.path],
    queryFn: () => getWorkspaceDirectory(entry.path),
    enabled: expanded,
    staleTime: 5000,
  })

  return (
    <div role="treeitem" aria-expanded={expanded}>
      <button
        type="button"
        className="hover:bg-accent focus-visible:ring-ring flex h-8 w-full min-w-0 items-center gap-1.5 rounded-md pr-2 text-left text-sm outline-none focus-visible:ring-2"
        style={{ paddingLeft: depth * 12 + 6 }}
        onClick={() => setExpanded((value) => !value)}
        title={entry.path}
      >
        <IconChevronRight
          className={cn(
            "text-muted-foreground size-3.5 shrink-0 transition-transform",
            expanded && "rotate-90",
          )}
        />
        <EntryIcon entry={entry} expanded={expanded} />
        <span className="min-w-0 flex-1 truncate">{entry.name}</span>
        {directoryQuery.isFetching && (
          <IconLoader2
            className="text-muted-foreground size-3.5 shrink-0 animate-spin"
            aria-label={t("chat.workspace.loading")}
          />
        )}
      </button>

      {expanded && (
        <div role="group">
          {directoryQuery.isError ? (
            <TreeError depth={depth + 1} />
          ) : directoryQuery.data?.entries.length === 0 ? (
            <div
              className="text-muted-foreground py-1.5 pr-2 text-xs italic"
              style={{ paddingLeft: (depth + 1) * 12 + 26 }}
            >
              {t("chat.workspace.emptyFolder")}
            </div>
          ) : (
            directoryQuery.data?.entries.map((child) => (
              <WorkspaceTreeEntry
                key={child.path}
                entry={child}
                depth={depth + 1}
              />
            ))
          )}
        </div>
      )}
    </div>
  )
}

function WorkspaceTreeEntry({
  entry,
  depth,
}: {
  entry: WorkspaceEntry
  depth: number
}) {
  if (entry.type === "directory") {
    return <DirectoryEntry entry={entry} depth={depth} />
  }

  const size = formatFileSize(entry.size)
  return (
    <div
      role="treeitem"
      className="hover:bg-accent flex h-8 min-w-0 items-center gap-1.5 rounded-md pr-2 text-sm"
      style={{ paddingLeft: depth * 12 + 25 }}
      title={entry.path}
    >
      <EntryIcon entry={entry} />
      <span className="min-w-0 flex-1 truncate">{entry.name}</span>
      {size && (
        <span className="text-muted-foreground shrink-0 text-[10px]">
          {size}
        </span>
      )}
    </div>
  )
}

function WorkspacePanel({
  open,
  onClose,
}: {
  open: boolean
  onClose?: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const rootQuery = useQuery({
    queryKey: [...workspaceQueryKey, ""],
    queryFn: () => getWorkspaceDirectory(),
    enabled: open,
    staleTime: 5000,
  })

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: workspaceQueryKey })
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="border-border/60 flex h-12 shrink-0 items-center gap-2 border-b px-3">
        <IconFolder className="size-4 text-amber-500" />
        <span className="min-w-0 flex-1 truncate text-sm font-medium">
          {rootQuery.data?.name ?? t("chat.workspace.title")}
        </span>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={refresh}
              disabled={rootQuery.isFetching}
              aria-label={t("chat.workspace.refresh")}
            >
              <IconRefresh
                className={cn("size-4", rootQuery.isFetching && "animate-spin")}
              />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t("chat.workspace.refresh")}</TooltipContent>
        </Tooltip>
        {onClose && (
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onClose}
            aria-label={t("chat.workspace.close")}
          >
            <IconX className="size-4" />
          </Button>
        )}
      </div>

      <div className="min-h-0 flex-1 [scrollbar-gutter:stable] overflow-y-auto px-2 py-2">
        {rootQuery.isPending ? (
          <div className="text-muted-foreground flex h-24 items-center justify-center gap-2 text-sm">
            <IconLoader2 className="size-4 animate-spin" />
            {t("chat.workspace.loading")}
          </div>
        ) : rootQuery.isError ? (
          <div
            className="text-muted-foreground flex flex-col items-center gap-2 px-4 py-10 text-center text-sm"
            role="alert"
          >
            <IconAlertCircle className="text-destructive size-5" />
            <span>{t("chat.workspace.loadFailed")}</span>
            <Button variant="outline" size="sm" onClick={refresh}>
              {t("chat.workspace.retry")}
            </Button>
          </div>
        ) : rootQuery.data.entries.length === 0 ? (
          <div className="text-muted-foreground px-4 py-10 text-center text-sm">
            {t("chat.workspace.empty")}
          </div>
        ) : (
          <div role="tree" aria-label={t("chat.workspace.fileTree")}>
            {rootQuery.data.entries.map((entry) => (
              <WorkspaceTreeEntry key={entry.path} entry={entry} depth={0} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export function WorkspaceSidebar({
  id,
  open,
  onOpenChange,
  forceSheet = false,
}: {
  id?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  forceSheet?: boolean
}) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const asideRef = useRef<HTMLElement>(null)
  const resizeRef = useRef<{
    pointerId: number
    startX: number
    startWidth: number
  } | null>(null)
  const [sidebarWidth, setSidebarWidth] = useState(loadWorkspaceSidebarWidth)

  const clampSidebarWidth = (width: number) => {
    const containerWidth = asideRef.current?.parentElement?.clientWidth
    const maxWidth = Math.max(
      minWorkspaceSidebarWidth,
      Math.min(640, (containerWidth ?? globalThis.innerWidth) - minChatWidth),
    )
    return Math.round(
      Math.min(maxWidth, Math.max(minWorkspaceSidebarWidth, width)),
    )
  }

  const finishResize = () => {
    resizeRef.current = null
    document.body.style.cursor = ""
    document.body.style.userSelect = ""
  }

  useEffect(() => {
    return () => {
      document.body.style.cursor = ""
      document.body.style.userSelect = ""
    }
  }, [])

  useEffect(() => {
    globalThis.localStorage?.setItem(
      workspaceSidebarWidthKey,
      String(sidebarWidth),
    )
  }, [sidebarWidth])

  useEffect(() => {
    if (!open || forceSheet || isMobile) return

    const constrainWidth = () => {
      const containerWidth = asideRef.current?.parentElement?.clientWidth
      if (!containerWidth) return
      const maxWidth = Math.max(
        minWorkspaceSidebarWidth,
        Math.min(640, containerWidth - minChatWidth),
      )
      setSidebarWidth((width) =>
        Math.round(
          Math.min(maxWidth, Math.max(minWorkspaceSidebarWidth, width)),
        ),
      )
    }

    constrainWidth()
    globalThis.addEventListener("resize", constrainWidth)
    return () => globalThis.removeEventListener("resize", constrainWidth)
  }, [forceSheet, isMobile, open])

  if (forceSheet || isMobile) {
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent
          id={id}
          side="right"
          className="w-[88vw] gap-0 p-0"
          showCloseButton={false}
        >
          <SheetHeader className="sr-only">
            <SheetTitle>{t("chat.workspace.title")}</SheetTitle>
            <SheetDescription>
              {t("chat.workspace.description")}
            </SheetDescription>
          </SheetHeader>
          <WorkspacePanel open={open} onClose={() => onOpenChange(false)} />
        </SheetContent>
      </Sheet>
    )
  }

  if (!open) return null

  return (
    <aside
      ref={asideRef}
      id={id}
      className="border-border/70 bg-background animate-in slide-in-from-right-4 relative flex shrink-0 border-l duration-200"
      style={{ width: sidebarWidth }}
      aria-label={t("chat.workspace.title")}
    >
      <div
        role="separator"
        aria-label={t("chat.workspace.resize")}
        aria-orientation="vertical"
        aria-valuemin={minWorkspaceSidebarWidth}
        aria-valuemax={640}
        aria-valuenow={sidebarWidth}
        tabIndex={0}
        className="group absolute inset-y-0 -left-1 z-10 w-2 cursor-col-resize touch-none outline-none"
        onPointerDown={(event) => {
          event.currentTarget.setPointerCapture(event.pointerId)
          resizeRef.current = {
            pointerId: event.pointerId,
            startX: event.clientX,
            startWidth: sidebarWidth,
          }
          document.body.style.cursor = "col-resize"
          document.body.style.userSelect = "none"
        }}
        onPointerMove={(event) => {
          const resize = resizeRef.current
          if (!resize || resize.pointerId !== event.pointerId) return
          setSidebarWidth(
            clampSidebarWidth(
              resize.startWidth + resize.startX - event.clientX,
            ),
          )
        }}
        onPointerUp={(event) => {
          if (resizeRef.current?.pointerId !== event.pointerId) return
          event.currentTarget.releasePointerCapture(event.pointerId)
          finishResize()
        }}
        onPointerCancel={finishResize}
        onKeyDown={(event) => {
          if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return
          event.preventDefault()
          const direction = event.key === "ArrowLeft" ? 1 : -1
          const nextWidth = clampSidebarWidth(sidebarWidth + direction * 16)
          setSidebarWidth(nextWidth)
        }}
      >
        <span className="bg-border group-hover:bg-primary group-focus-visible:bg-primary absolute inset-y-0 left-1/2 w-px -translate-x-1/2 transition-colors" />
      </div>
      <WorkspacePanel open={open} />
    </aside>
  )
}
