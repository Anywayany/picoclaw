import {
  buildLauncherAuthPath,
  getCurrentMobileRedirectTarget,
  isMobilePathname,
} from "@/features/mobile/redirect"
import { isLauncherAuthPathname } from "@/lib/launcher-login-path"
import {
  stripBasePath,
  withBasePath,
  withBasePathInput,
} from "@/lib/public-base-path"

function isLauncherAuthPath(): boolean {
  if (typeof globalThis.location === "undefined") {
    return false
  }
  if (
    isLauncherAuthPathname(stripBasePath(globalThis.location.pathname || "/"))
  ) {
    return true
  }
  try {
    return isLauncherAuthPathname(
      stripBasePath(new URL(globalThis.location.href).pathname || "/"),
    )
  } catch {
    return false
  }
}

/**
 * Same-origin fetch that sends cookies; redirects to launcher login on 401 JSON responses.
 * Skips redirect while already on an auth page (login or setup) to avoid reload loops.
 */
export async function launcherFetch(
  input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> {
  const res = await fetch(withBasePathInput(input), {
    credentials: "same-origin",
    ...init,
  })
  if (res.status === 401) {
    const ct = res.headers.get("content-type") || ""
    if (
      ct.includes("application/json") &&
      typeof globalThis.location !== "undefined" &&
      !isLauncherAuthPath()
    ) {
      const pathname = stripBasePath(globalThis.location.pathname || "/")
      globalThis.location.assign(
        isMobilePathname(pathname)
          ? buildLauncherAuthPath(
              "/launcher-login",
              getCurrentMobileRedirectTarget(),
            )
          : withBasePath("/launcher-login"),
      )
    }
  }
  return res
}
