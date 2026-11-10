import { useSyncExternalStore } from 'react'
import { useLocation } from './router'
import { useMe } from './session'

// English / Urdu. Every screen text is written as t('English', 'اردو').
export type Lang = 'en' | 'ur'

// The choice is kept per kind of visitor, so a worker switching to English (or a customer to Urdu)
// never changes what the other sees on the same device. Defaults: workers Urdu, everyone else English.
type Who = 'worker' | 'customer' | 'guest'
const key = (who: Who) => `rr:lang:${who}`
const stored = (who: Who): Lang | null => {
  try { const v = localStorage.getItem(key(who)); return v === 'en' || v === 'ur' ? v : null } catch { return null }
}
const listeners = new Set<() => void>()
let chosen: Record<Who, Lang | null> = { worker: stored('worker'), customer: stored('customer'), guest: stored('guest') }

function setLang(who: Who, lang: Lang) {
  chosen = { ...chosen, [who]: lang }
  try { localStorage.setItem(key(who), lang) } catch { /* private mode: kept for this visit only */ }
  listeners.forEach((l) => l())
}

/** Signed in: the account's role. Signed out: the worker pages count as a worker, anything else as a guest. */
function useWho(): Who {
  const me = useMe()
  const { path } = useLocation()
  return me?.user.role ?? (path.startsWith('/worker') ? 'worker' : 'guest')
}

/** The visitor's own choice for their role; until they choose, workers get Urdu and customers and guests English. */
export function useLang(): Lang {
  const all = useSyncExternalStore((l) => (listeners.add(l), () => void listeners.delete(l)), () => chosen)
  const who = useWho()
  return all[who] ?? (who === 'worker' ? 'ur' : 'en')
}

/** const t = useT(); t('Sign in', 'سائن اِن'). Call it at the top of the component, before any early return. */
export function useT() {
  const lang = useLang()
  return (en: string, ur: string) => (lang === 'ur' ? ur : en)
}

/** One button that switches to the other language; it is labelled in the language it switches to. */
export function LangSwitch() {
  const lang = useLang()
  const who = useWho()
  return (
    <button className="lang-switch" type="button" lang={lang === 'ur' ? 'en' : 'ur'} dir={lang === 'ur' ? 'ltr' : 'rtl'}
      aria-label={lang === 'ur' ? 'Switch to English' : 'اردو میں دیکھیں'} onClick={() => setLang(who, lang === 'ur' ? 'en' : 'ur')}>
      {lang === 'ur' ? 'English' : 'اردو'}
    </button>
  )
}
