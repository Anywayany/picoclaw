import { useEffect } from "react"

import { subscribeGatewayPolling } from "@/store/gateway"

export function useMobileGatewayPolling() {
  useEffect(() => subscribeGatewayPolling(), [])
}
