import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react'
import type { User } from '../contract'
import { signOut as firebaseSignOut } from 'firebase/auth'
import { api, MOCK, type Me } from './api'
import { auth, DEV, setDevUser } from './firebase'
import { mockSignOut } from './mocks'
import { navigate } from './router'

// Signed-in user, kept for the tab so a reload does not sign out.
const KEY = 'rr:me'
let me: Me | null = (() => { try { return JSON.parse(sessionStorage.getItem(KEY) ?? 'null') as Me | null } catch { return null } })()
const listeners = new Set<() => void>()
function set(next: Me | null) {
  me = next
  try { sessionStorage.setItem(KEY, JSON.stringify(next)) } catch { /* private mode: memory only */ }
  listeners.forEach((l) => l())
}

export const useMe = () => useSyncExternalStore((l) => (listeners.add(l), () => void listeners.delete(l)), () => me)
export const setMe = set
// The API answered 401: the stored user is no longer signed in.
window.addEventListener('rr:signed-out', () => { if (me) set(null) })

/** True when sign-in screens skip Firebase: the mock backend, or dev tokens against a local API. */
export const FAKE_AUTH = MOCK || !!DEV

/**
 * Opens the API session. Call it after every Firebase sign-in (the API creates the user on first call).
 * keep=false returns the account without storing it, so a sign-in page can check the role first:
 * storing it sends the visitor straight on, because signed-in visitors skip the sign-in pages.
 */
export async function signIn(role: User['role'], keep = true) {
  if (DEV) setDevUser(role)
  const next = await api.session(role)
  if (keep) set(next)
  return next
}

/** Where to go after sign-in: only a path on this site ("/x", not "//host" or "/\host"); anything else goes home. */
export const safeNext = (next: string | null) => (next && /^\/(?![/\\])/.test(next) ? next : '/app')

export function signOut() {
  if (MOCK) mockSignOut()
  else if (!DEV) void firebaseSignOut(auth())
  set(null)
  navigate('/')
}

/** Sends visitors without the right role to that role's sign-in page. Returns the session once it fits. */
export function useRole(role: User['role']): Me | null {
  const current = useMe()
  const ok = current?.user.role === role
  const back = location.pathname + location.search
  useEffect(() => {
    // Signed out: sign in first. Signed in with the other role: back to their own app home.
    if (!ok) navigate(current ? '/app' : role === 'worker' ? '/worker/login' : `/login?next=${encodeURIComponent(back)}`, true)
  }, [ok, role, back, current])
  return ok ? current : null
}

export type Load<T> = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ready'; data: T }

/** Runs a request on mount (and when `key` changes); `reload` retries. `poll` refetches quietly every n ms. */
export function useLoad<T>(fn: () => Promise<T>, key: unknown[], poll?: number) {
  const [state, setState] = useState<Load<T>>({ status: 'loading' })
  const [tick, setTick] = useState(0)
  const quiet = useRef(false)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const run = useCallback(fn, key)
  useEffect(() => {
    let live = true
    const go = (quiet: boolean) => {
      if (!quiet) setState({ status: 'loading' })
      run().then((data) => live && setState({ status: 'ready', data }), (error: unknown) => live && !quiet && setState({ status: 'error', error }))
    }
    go(quiet.current)
    quiet.current = false
    // shortcut: polling stands in for the WebSocket events (job.updated, message.created) until the hub exists.
    const timer = poll ? setInterval(() => go(true), poll) : undefined
    return () => { live = false; clearInterval(timer) }
  }, [run, tick, poll])
  // A refetch keeps showing the current data; only a retry after an error shows the loading state.
  const reload = () => { quiet.current = state.status === 'ready'; setTick((t) => t + 1) }
  return { state, reload }
}
