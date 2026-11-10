import { useEffect, useSyncExternalStore, type AnchorHTMLAttributes, type MouseEvent } from 'react'

// Minimal pushState router: the site has ~20 flat routes, so no routing library.
const listeners = new Set<() => void>()
window.addEventListener('popstate', () => listeners.forEach((l) => l()))

export function navigate(to: string, replace = false) {
  history[replace ? 'replaceState' : 'pushState'](null, '', to)
  listeners.forEach((l) => l())
  const hash = to.split('#')[1]
  if (hash) setTimeout(() => document.getElementById(hash)?.scrollIntoView())
  else window.scrollTo(0, 0)
}

const subscribe = (l: () => void) => (listeners.add(l), () => void listeners.delete(l))
const snapshot = () => location.pathname + location.search

export function useLocation() {
  const href = useSyncExternalStore(subscribe, snapshot)
  const url = new URL(href, location.origin)
  return { path: url.pathname, query: url.searchParams }
}

/** Matches '/jobs/:id/offer' style patterns; returns params or null. */
export function match(pattern: string, path: string): Record<string, string> | null {
  const p = pattern.split('/'), s = path.replace(/\/$/, '').split('/')
  if (path === '/' && pattern === '/') return {}
  if (p.length !== s.length) return null
  const params: Record<string, string> = {}
  for (let i = 0; i < p.length; i++) {
    if (p[i].startsWith(':')) params[p[i].slice(1)] = decodeURIComponent(s[i])
    else if (p[i] !== s[i]) return null
  }
  return params
}

export function Link({ href, onClick, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) {
  const go = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e)
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
    e.preventDefault()
    navigate(href)
  }
  return <a href={href} onClick={go} {...rest} />
}

/** Replaces the current page once rendered. */
export function Redirect({ to }: { to: string }) {
  useEffect(() => navigate(to, true), [to])
  return null
}
