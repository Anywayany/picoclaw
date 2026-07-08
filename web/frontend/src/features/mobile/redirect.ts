import { stripBasePath, withBasePath } from "@/lib/public-base-path"

const DEFAULT_REDIRECT_FALLBACK = "/"

function hasControlCharacter(value: string): boolean {
  return Array.from(value).some((char) => {
    const code = char.charCodeAt(0)
    return code <= 0x1f || code === 0x7f
  })
}

export function getSafeRedirectTarget(
  value: string | null | undefined,
  fallback = withBasePath(DEFAULT_REDIRECT_FALLBACK),
): string {
  const target = value?.trim()
  if (!target) {
    return fallback
  }
  if (!target.startsWith("/") || target.startsWith("//")) {
    return fallback
  }
  if (hasControlCharacter(target)) {
    return fallback
  }
  try {
    const parsed = new URL(
      target,
      globalThis.location?.origin ?? "http://localhost",
    )
    if (parsed.origin !== (globalThis.location?.origin ?? parsed.origin)) {
      return fallback
    }
    return `${withBasePath(parsed.pathname)}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}

export function getRedirectTargetFromLocation(
  fallback = withBasePath(DEFAULT_REDIRECT_FALLBACK),
): string {
  if (typeof globalThis.location === "undefined") {
    return fallback
  }
  const params = new URLSearchParams(globalThis.location.search)
  return getSafeRedirectTarget(params.get("redirect"), fallback)
}

export function buildLauncherAuthPath(
  pathname: "/launcher-login" | "/launcher-setup",
  redirectTarget: string,
): string {
  const safeRedirect = getSafeRedirectTarget(redirectTarget)
  const authPath = withBasePath(pathname)
  if (safeRedirect === withBasePath(DEFAULT_REDIRECT_FALLBACK)) {
    return authPath
  }
  return `${authPath}?redirect=${encodeURIComponent(safeRedirect)}`
}

export function isMobilePathname(pathname: string): boolean {
  const stripped = stripBasePath(pathname)
  return stripped === "/mobile" || stripped.startsWith("/mobile/")
}

export function getCurrentMobileRedirectTarget(): string {
  if (typeof globalThis.location === "undefined") {
    return withBasePath("/mobile")
  }
  const { pathname, search, hash } = globalThis.location
  if (!isMobilePathname(pathname || "/")) {
    return withBasePath("/mobile")
  }
  return `${withBasePath(pathname || "/mobile")}${search}${hash}`
}
