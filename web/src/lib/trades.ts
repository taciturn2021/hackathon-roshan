import type { IconName } from '../components/Icon'
import type { JobStatus, Trade, Urgency } from '../contract'

export const TRADES: Record<Trade, { icon: IconName; en: string; ur: string; blurb: string }> = {
  electrician: { icon: 'Zap', en: 'Electrician', ur: 'بجلی', blurb: 'Lights, switches & wiring' },
  plumber: { icon: 'Droplets', en: 'Plumber', ur: 'پانی', blurb: 'Leaks, taps & pipes' },
  ac_technician: { icon: 'Snowflake', en: 'AC technician', ur: 'اے سی', blurb: 'Cooling, cleaning & care' },
  carpenter: { icon: 'Hammer', en: 'Carpenter', ur: 'بڑھئی', blurb: 'Doors, furniture & fittings' },
  mechanic: { icon: 'Wrench', en: 'Mechanic', ur: 'مکینک', blurb: 'Everyday mechanical repairs' },
}
export const TRADE_KEYS = Object.keys(TRADES) as Trade[]

export const URGENCY: Record<Urgency, { en: string; ur: string }> = {
  now: { en: 'Now', ur: 'ابھی' },
  today: { en: 'Today', ur: 'آج' },
  flexible: { en: 'Flexible', ur: 'کبھی بھی' },
}

export const STATUS: Record<JobStatus, { en: string; ur: string; tone: 'warning' | 'success' | 'error' }> = {
  requested: { en: 'Requested', ur: 'درخواست', tone: 'warning' },
  confirmed: { en: 'Confirmed', ur: 'پکا', tone: 'success' },
  in_progress: { en: 'In progress', ur: 'جاری', tone: 'success' },
  completed: { en: 'Completed', ur: 'مکمل', tone: 'success' },
  cancelled: { en: 'Cancelled', ur: 'منسوخ', tone: 'error' },
}

export const TAGS: Record<string, string> = { on_time: 'On time', fair_price: 'Fair price', clean_work: 'Clean work', polite: 'Polite' }

export const rupees = (n: number) => 'Rs ' + n.toLocaleString('en-PK')
export const initials = (name: string) => name.split(/\s+/).map((p) => p[0]).slice(0, 2).join('').toUpperCase()
export const km = (d: number | null) => (d === null ? null : `${d.toFixed(1)} km`)
export const isActive = (s: JobStatus) => s === 'requested' || s === 'confirmed' || s === 'in_progress'
