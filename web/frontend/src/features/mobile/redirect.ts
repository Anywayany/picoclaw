const DEFAULT_REDIRECT_FALLBACK = "/"

function hasControlCharacter(value: string): boolean {
  return Array.from(value).some((char) => {
    const code = char.charCodeAt(0)
    return code <= 0x1f || code === 0x7f
  })
}

export function getSafeRedirectTarget(
  value: string | null | undefined,
  fallback = DEFAULT_REDIRECT_FALLBACK,
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
    const parsed = new URL(target, globalThis.location?.origin ?? "http://localhost")
    if (parsed.origin !== (globalThis.location?.origin ?? parsed.origin)) {
      return fallback
    }
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return fallback
  }
}

export function getRedirectTargetFromLocation(
  fallback = DEFAULT_REDIRECT_FALLBACK,
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
  if (safeRedirect === DEFAULT_REDIRECT_FALLBACK) {
    return pathname
  }
  return `${pathname}?redirect=${encodeURIComponent(safeRedirect)}`
}

export function isMobilePathname(pathname: string): boolean {
  return pathname === "/mobile" || pathname.startsWith("/mobile/")
}

export function getCurrentMobileRedirectTarget(): string {
  if (typeof globalThis.location === "undefined") {
    return "/mobile"
  }
  const { pathname, search, hash } = globalThis.location
  if (!isMobilePathname(pathname || "/")) {
    return "/mobile"
  }
  return `${pathname || "/mobile"}${search}${hash}`
}
