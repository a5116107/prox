/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

const API_REWRITE_PREFIXES = ['/api', '/v1', '/mj', '/pg']

export function getApiBaseURL(): string {
  if (typeof window === 'undefined') return ''
  const { protocol, hostname, port } = window.location
  const lowerHost = hostname.toLowerCase()
  if (!lowerHost.startsWith('ai.')) return ''
  const apiHost = `api.${hostname.slice(3)}`
  const portPart = port ? `:${port}` : ''
  return `${protocol}//${apiHost}${portPart}`
}

function shouldRewritePath(pathname: string): boolean {
  return API_REWRITE_PREFIXES.some((prefix) => {
    return pathname === prefix || pathname.startsWith(`${prefix}/`)
  })
}

function rewriteUrlLike(rawUrl: string): string | null {
  const apiBase = getApiBaseURL()
  if (!apiBase) return null

  try {
    if (rawUrl.startsWith('/')) {
      const url = new URL(rawUrl, window.location.origin)
      if (shouldRewritePath(url.pathname))
        return `${apiBase}${url.pathname}${url.search}${url.hash}`
      return null
    }

    const url = new URL(rawUrl, window.location.href)
    if (url.origin !== window.location.origin) return null
    if (!shouldRewritePath(url.pathname)) return null
    return `${apiBase}${url.pathname}${url.search}${url.hash}`
  } catch {
    return null
  }
}

export function installApiOriginFetchRewrite(): void {
  if (typeof window === 'undefined') return
  const marker = '__newApiFetchOriginRewriteInstalled__'
  const win = window as typeof window & Record<string, unknown>
  if (win[marker]) return
  win[marker] = true

  const nativeFetch = window.fetch.bind(window)
  window.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    if (typeof input === 'string') {
      const next = rewriteUrlLike(input)
      return nativeFetch(next ?? input, init)
    }

    if (input instanceof URL) {
      const next = rewriteUrlLike(input.toString())
      return nativeFetch(next ? new URL(next) : input, init)
    }

    if (input instanceof Request) {
      const next = rewriteUrlLike(input.url)
      if (next) return nativeFetch(new Request(next, input), init)
    }

    return nativeFetch(input, init)
  }) as typeof window.fetch
}
