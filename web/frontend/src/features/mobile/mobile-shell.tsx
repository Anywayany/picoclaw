import type { ReactNode } from "react"
import { Toaster } from "sonner"

import { TooltipProvider } from "@/components/ui/tooltip"

interface MobileShellProps {
  children: ReactNode
  authError: string | null
  onDismissAuthError: () => void
  devtools?: ReactNode
}

export function MobileShell({
  children,
  authError,
  onDismissAuthError,
  devtools,
}: MobileShellProps) {
  return (
    <TooltipProvider>
      <main className="bg-background text-foreground flex h-dvh min-h-dvh w-full flex-col overflow-hidden">
        {authError && (
          <div className="bg-destructive text-destructive-foreground fixed inset-x-0 top-0 z-[100] flex items-center justify-between px-4 py-2 text-sm shadow-md">
            <span>Auth service error: {authError}</span>
            <button
              className="ml-4 opacity-70 hover:opacity-100"
              onClick={onDismissAuthError}
              aria-label="Dismiss"
            >
              x
            </button>
          </div>
        )}
        {children}
        {devtools}
        <Toaster position="bottom-center" />
      </main>
    </TooltipProvider>
  )
}
