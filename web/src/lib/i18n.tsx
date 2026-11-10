import { useSyncExternalStore } from 'react'
import { useLocation } from './router'
import { useMe } from './session'

// English / Urdu. Every screen text is written as t('English', 'اردو').
export type Lang = 'en' | 'ur'
const KEY = 'rr:lang'
const listeners = new Set<() => void>()
let chosen: Lang | null = (() => {
  try { const v = localStorage.getItem(KEY); return v === 'en' || v === 'ur' ? v : null } catch { return null }
})()

export function setLang(lang: Lang) {
  chosen = lang
  try { localStorage.setItem(KEY, lang) } catch { /* private mode: kept for this visit only */ }
  listeners.forEach((l) => l())
}

/** The visitor's own choice. Until they choose, workers (and the worker pages) get Urdu and everyone else English. */
export function useLang(): Lang {
  const choice = useSyncExternalStore((l) => (listeners.add(l), () => void listeners.delete(l)), () => chosen)
  const me = useMe()
  const { path } = useLocation()
  return choice ?? (me?.user.role === 'worker' || path.startsWith('/worker') ? 'ur' : 'en')
}

/** const t = useT(); t('Sign in', 'سائن اِن'). Call it at the top of the component, before any early return. */
export function useT() {
  const lang = useLang()
  return (en: string, ur: string) => (lang === 'ur' ? ur : en)
}

/** One button that switches to the other language; it is labelled in the language it switches to. */
export function LangSwitch() {
  const lang = useLang()
  return (
    <button className="lang-switch" type="button" lang={lang === 'ur' ? 'en' : 'ur'} dir={lang === 'ur' ? 'ltr' : 'rtl'}
      aria-label={lang === 'ur' ? 'Switch to English' : 'اردو میں دیکھیں'} onClick={() => setLang(lang === 'ur' ? 'en' : 'ur')}>
      {lang === 'ur' ? 'English' : 'اردو'}
    </button>
  )
}
