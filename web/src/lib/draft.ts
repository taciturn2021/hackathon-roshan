// Drafts that survive navigation and reloads within the tab, so a failure never loses what was entered.
import type { OnboardAnswer, PriceRange, Trade, Urgency } from '../contract'

export type ProblemDraft = {
  description: string
  trade: Trade | null
  trade_confidence: number
  urgency: Urgency
  price_range: PriceRange
  lat: number | null
  lng: number | null
  radius_km: number
}
export type OnboardDraft = {
  profile: OnboardAnswer['profile']
  intro_audio_url: string | null
  lat: number | null
  lng: number | null
}

function store<T>(key: string, empty: T) {
  return {
    get(): T {
      try { return { ...empty, ...(JSON.parse(sessionStorage.getItem(key) ?? '{}') as Partial<T>) } } catch { return empty }
    },
    set(patch: Partial<T>): T {
      const next = { ...this.get(), ...patch }
      try { sessionStorage.setItem(key, JSON.stringify(next)) } catch { /* memory-only browsers lose the draft on reload */ }
      return next
    },
    clear() { try { sessionStorage.removeItem(key) } catch { /* nothing stored */ } },
  }
}

export const problemDraft = store<ProblemDraft>('rr:problem', {
  description: '', trade: null, trade_confidence: 0, urgency: 'now', price_range: null, lat: null, lng: null, radius_km: 5,
})
export const onboardDraft = store<OnboardDraft>('rr:onboard', { profile: {}, intro_audio_url: null, lat: null, lng: null })

/** Browser location as a promise; rejects when denied or unavailable. */
export const locate = () => new Promise<{ lat: number; lng: number }>((resolve, reject) => {
  if (!navigator.geolocation) return reject(new Error('unavailable'))
  navigator.geolocation.getCurrentPosition((p) => resolve({ lat: p.coords.latitude, lng: p.coords.longitude }), reject, { timeout: 15000 })
})
