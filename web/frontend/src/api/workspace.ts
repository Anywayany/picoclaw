import { launcherFetch } from "@/api/http"

export type WorkspaceEntryType = "directory" | "file" | "symlink"

export interface WorkspaceEntry {
  name: string
  path: string
  type: WorkspaceEntryType
  size?: number
  modified?: string
}

export interface WorkspaceDirectory {
  name: string
  path: string
  entries: WorkspaceEntry[]
}

export async function getWorkspaceDirectory(
  path = "",
): Promise<WorkspaceDirectory> {
  const params = new URLSearchParams()
  if (path) {
    params.set("path", path)
  }
  const query = params.size > 0 ? `?${params.toString()}` : ""
  const response = await launcherFetch(`/api/workspace${query}`)
  if (!response.ok) {
    const message = (await response.text()).trim()
    throw new Error(message || `Failed to load workspace: ${response.status}`)
  }
  return response.json() as Promise<WorkspaceDirectory>
}
