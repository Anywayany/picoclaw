function normalizeBasePath(raw: string | undefined): string {
  const trimmed = (raw ?? "").trim()
  if (trimmed === "" || trimmed === "/") {
    return ""
  }
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`
  return withSlash.replace(/\/+$/, "")
}

export const PUBLIC_BASE_PATH = normalizeBasePath(
  import.meta.env.VITE_PUBLIC_BASE_PATH,
)

export function withBasePath(path: string): string {
  if (PUBLIC_BASE_PATH === "") {
    return path
  }
  if (!path.startsWith("/") || path.startsWith("//")) {
    return path
  }
  if (path === PUBLIC_BASE_PATH || path.startsWith(`${PUBLIC_BASE_PATH}/`)) {
    return path
  }
  if (path === "/") {
    return `${PUBLIC_BASE_PATH}/`
  }
  return `${PUBLIC_BASE_PATH}${path}`
}

export function stripBasePath(pathname: string): string {
  if (PUBLIC_BASE_PATH === "") {
    return pathname || "/"
  }
  if (pathname === PUBLIC_BASE_PATH) {
    return "/"
  }
  if (pathname.startsWith(`${PUBLIC_BASE_PATH}/`)) {
    return pathname.slice(PUBLIC_BASE_PATH.length) || "/"
  }
  return pathname || "/"
}

export function withBasePathInput(input: RequestInfo | URL): RequestInfo | URL {
  if (typeof input === "string") {
    return withBasePath(input)
  }
  if (input instanceof URL) {
    if (
      typeof globalThis.location !== "undefined" &&
      input.origin === globalThis.location.origin
    ) {
      const copy = new URL(input.href)
      copy.pathname = withBasePath(copy.pathname)
      return copy
    }
    return input
  }
  return input
}
