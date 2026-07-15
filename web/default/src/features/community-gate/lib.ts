/*
Copyright (C) 2023-2026 QuantumNous
*/
import type { CommunityGateMeStatus } from './types'

const COMMUNITY_GATE_AUTO_OPEN_COOLDOWN_MS = 6 * 60 * 60 * 1000

type CommunityGateAutoOpenTarget = {
  url: string
  reason: 'join'
}

function resolveCommunityGateAutoOpenTarget(
  status: CommunityGateMeStatus | null
): CommunityGateAutoOpenTarget | null {
  if (!status?.gate?.enabled || status.compliant) return null
  if (!status.gate.has_room_membership && status.join_url) {
    return { url: status.join_url, reason: 'join' }
  }
  return null
}

export function maybeAutoOpenCommunityGate(
  status: CommunityGateMeStatus | null,
  userId?: number
): boolean {
  if (typeof window === 'undefined') return false

  const target = resolveCommunityGateAutoOpenTarget(status)
  if (!target) return false

  const roomPart = status?.room_id || status?.gate?.room_id || 'default'
  const cooldownKey = `community-gate-auto-open:${userId || 'unknown'}:${roomPart}:${target.reason}`

  try {
    const prev = Number(window.sessionStorage.getItem(cooldownKey) || '0')
    if (Date.now() - prev < COMMUNITY_GATE_AUTO_OPEN_COOLDOWN_MS) {
      return false
    }
    window.sessionStorage.setItem(cooldownKey, String(Date.now()))
  } catch {
    // ignore storage failures
  }

  try {
    window.location.replace(target.url)
    return true
  } catch {
    try {
      window.location.href = target.url
      return true
    } catch {
      return false
    }
  }
}
