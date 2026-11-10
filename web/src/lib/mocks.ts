// MOCK BACKEND. Fictional people and jobs, used only while VITE_MOCK_API is not 'false'.
// It implements the same contract as the Go API so every screen can be exercised end to end.
// Data lives in sessionStorage, so the demo resets when the tab closes.
import type { Job, Message, OnboardField, PriceRange, Review, Trade, User, WorkerCard, WorkerDetail } from '../contract'
import { ApiError, type Api, type Me } from './api'

type State = {
  role: User['role'] | null
  onboarded: boolean
  workers: WorkerDetail[]
  jobs: Job[]
  messages: Message[]
  nextId: number
}

const customer: User = { id: 100, email: 'ayesha@example.com', phone: null, role: 'customer', name: 'Ayesha', photo_url: null }
const workerUser: User = { id: 1, email: 'aslam@example.com', phone: null, role: 'worker', name: 'Muhammad Aslam', photo_url: null }

const ranges: Record<Trade, PriceRange> = {
  ac_technician: { low: 1200, high: 2500, sample: 18 },
  electrician: { low: 800, high: 2000, sample: 22 },
  plumber: { low: 700, high: 1800, sample: 15 },
  carpenter: { low: 1000, high: 3000, sample: 9 },
  mechanic: { low: 1000, high: 2500, sample: 12 },
}

const w = (id: number, name: string, trade: Trade, area: string, years: number, km: number, rating: number, jobs: number, summary: string, tags: string[], reviews: Review[] = []): WorkerDetail => ({
  id, name, trade, area, years_exp: years, photo_url: null, intro_audio_url: null, is_online: true, available: true,
  distance_km: km, rating_avg: rating, jobs_done: jobs, trust_summary: summary, tags, reviews, price_range: ranges[trade],
})

function seed(): State {
  const workers = [
      w(1, 'Muhammad Aslam', 'ac_technician', 'G-10, Islamabad', 8, 1.2, 4.8, 36, 'Customers mention punctual visits, clear pricing and tidy work.', ['on_time', 'fair_price', 'clean_work'],
        [{ job_id: 1, rating: 5, body: 'Explained the problem clearly and left the room tidy.', audio_url: null, tags: ['clean_work'], created_at: '2026-11-08T10:00:00Z' }]),
      w(2, 'Imran Ali', 'ac_technician', 'G-11, Islamabad', 6, 2.4, 4.7, 24, 'Customers mention quick diagnosis and fair prices.', ['fair_price']),
      w(3, 'Shahid Khan', 'ac_technician', 'G-9, Islamabad', 12, 3.1, 4.9, 42, 'Customers mention polite, careful work.', ['polite', 'clean_work']),
      w(4, 'Bilal Ahmed', 'electrician', 'F-8, Islamabad', 5, 1.8, 4.6, 19, 'Customers mention safe, neat wiring.', ['on_time']),
      w(5, 'Usman Tariq', 'plumber', 'G-11, Islamabad', 9, 2.0, 4.7, 28, 'Customers mention leaks fixed on the first visit.', ['fair_price', 'on_time']),
      w(6, 'Kashif Mehmood', 'carpenter', 'I-8, Islamabad', 15, 3.6, 4.5, 15, 'Customers mention sturdy, well-finished repairs.', ['clean_work']),
      w(7, 'Naveed Akhtar', 'mechanic', 'Saddar, Rawalpindi', 10, 2.9, 4.8, 31, 'Customers mention honest advice about repairs.', ['fair_price', 'polite']),
  ]
  // Past jobs, so dashboards and job history have something to show.
  const neighbour: User = { id: 101, email: 'farah@example.com', phone: null, role: 'customer', name: 'Farah', photo_url: null }
  const past = (id: number, who: User, workerId: number, description: string, price: number, day: string, reviewed: boolean): Job => {
    const d = workers.find((x) => x.id === workerId)!
    return {
      id, customer: who, worker: card(d), trade: d.trade, description, urgency: 'today', photo_url: null, status: 'completed',
      agreed_price: price, brief_ur: null, brief_audio_url: null, created_at: `${day}T09:00:00Z`, completed_at: `${day}T13:00:00Z`, reviewed,
    }
  }
  const jobs = [
    past(1, customer, 1, 'AC running but the room stays warm.', 1500, '2026-11-08', true),
    past(4, neighbour, 1, 'Split AC needs a gas refill.', 2000, '2026-11-06', true),
    past(2, customer, 5, 'Kitchen tap leaking under the sink.', 1200, '2026-11-02', true),
    past(3, customer, 4, 'Two switches sparking in the lounge.', 900, '2026-10-27', false),
  ]
  return { role: null, onboarded: false, jobs, messages: [], nextId: 1000, workers }
}

const KEY = 'rr:mock'
const load = (): State => {
  try { return JSON.parse(sessionStorage.getItem(KEY) ?? '') as State } catch { return seed() }
}
const db = load()
const save = () => { try { sessionStorage.setItem(KEY, JSON.stringify(db)) } catch { /* storage full or blocked: keep in memory */ } }
const wait = <T,>(value: T, ms = 450) => new Promise<T>((r) => setTimeout(() => r(structuredClone(value)), ms))
const now = () => new Date().toISOString()
const id = () => ++db.nextId

const me = (): User => {
  if (!db.role) throw new ApiError(401, 'not signed in')
  return db.role === 'worker' ? workerUser : customer
}
function card(d: WorkerDetail): WorkerCard {
  const { reviews: _r, price_range: _p, ...c } = d
  return c
}
const findJob = (jobId: number) => {
  const job = db.jobs.find((j) => j.id === jobId)
  const u = me()
  if (!job || (job.customer.id !== u.id && job.worker.id !== u.id)) throw new ApiError(404, 'job not found')
  return job
}
const refreshWorker = (job: Job) => {
  const detail = db.workers.find((x) => x.id === job.worker.id)!
  const active = db.jobs.some((j) => j.worker.id === detail.id && (j.status === 'confirmed' || j.status === 'in_progress'))
  detail.available = !active
  db.jobs.forEach((j) => { if (j.worker.id === detail.id) j.worker = card(detail) })
}
const message = (jobId: number, sender: number, m: Partial<Message>): Message => ({
  id: id(), job_id: jobId, sender_id: sender, kind: 'text', body: null, audio_url: null, body_ur: null, speech_url: null,
  suggested_price: null, offer_price: null, offer_status: null, created_at: now(), ...m,
})
const tradeUr: Record<Trade, string> = { electrician: 'بجلی', plumber: 'پانی', ac_technician: 'اے سی', carpenter: 'بڑھئی', mechanic: 'مکینک' }

const keywords: [Trade, RegExp][] = [
  ['ac_technician', /\bac\b|a\.c|air ?con|cool|thand/i],
  ['electrician', /light|switch|wir|electric|bijli|fan|socket|power/i],
  ['plumber', /leak|tap|pipe|plumb|water|pani|drain|toilet|geyser/i],
  ['carpenter', /door|wood|furniture|cupboard|carpent|lakri|almari|drawer/i],
  ['mechanic', /\bcar\b|bike|engine|mechanic|motor|generator/i],
]

const sampleAnswers: Record<OnboardField, WorkerProfile> = {
  name: { name: 'Muhammad Aslam' },
  trade: { trade: 'ac_technician', skills: ['AC cleaning', 'Gas refill'] },
  area: { area: 'G-10, Islamabad' },
  years_exp: { years_exp: 8 },
}
type WorkerProfile = { name?: string; trade?: Trade; skills?: string[]; area?: string; years_exp?: number }

export const mockApi: Api = {
  session: (role) => {
    db.role = role
    save()
    return mySession()
  },
  me: () => mySession(),
  onboard: (_audio, field) => wait({ field, profile: sampleAnswers[field], confidence: 0.9, transcript: '', audio_url: '' }, 900),
  saveWorker: (p) => {
    const d = db.workers.find((x) => x.id === workerUser.id)!
    Object.assign(d, { name: p.name, trade: p.trade, years_exp: p.years_exp, area: p.area, intro_audio_url: p.intro_audio_url || null, price_range: ranges[p.trade] })
    db.onboarded = true
    save()
    return wait(card(d))
  },
  setOnline: (is_online) => {
    const d = db.workers.find((x) => x.id === workerUser.id)!
    d.is_online = is_online
    save()
    return wait(card(d), 200)
  },
  listWorkers: (q) => wait(db.workers
    .filter((x) => x.trade === q.trade && x.is_online && x.available && (x.distance_km ?? Infinity) <= q.radius_km)
    .sort((a, b) => (a.distance_km ?? 0) - (b.distance_km ?? 0)).map(card), 700),
  getWorker: (workerId) => {
    const d = db.workers.find((x) => x.id === workerId)
    return d ? wait(d) : Promise.reject(new ApiError(404, 'worker not found'))
  },
  analyzeProblem: ({ text = '' }) => {
    const hit = keywords.find(([, re]) => re.test(text))
    const trade = hit?.[0] ?? null
    const urgency = /\bnow\b|urgent|abhi|asap|immediately/i.test(text) ? 'now' : /today|aaj/i.test(text) ? 'today' : 'flexible'
    return wait({ trade, trade_confidence: trade ? 0.9 : 0.2, urgency, description: text, photo_url: null, price_range: trade ? ranges[trade] : null }, 900)
  },
  createJob: (j) => {
    const u = me(), d = db.workers.find((x) => x.id === j.worker_id)
    if (!d || !d.available || !d.is_online) return Promise.reject(new ApiError(409, 'worker_busy'))
    const job: Job = {
      id: id(), customer: u, worker: card(d), trade: j.trade, description: j.description, urgency: j.urgency, photo_url: null,
      status: 'requested', agreed_price: null, brief_ur: `گاہک کو ${tradeUr[j.trade]} کے کام کے لیے مدد چاہیے۔`, brief_audio_url: null,
      created_at: now(), completed_at: null, reviewed: false,
    }
    db.jobs.unshift(job)
    // Stand-in for the worker's side so a customer can try the whole flow alone.
    const price = (ranges[j.trade]?.low ?? 1000) + 300
    db.messages.push(
      message(job.id, d.id, { body: `Salam, I can come and take a look. My price is Rs ${price.toLocaleString('en-PK')}.` }),
      message(job.id, d.id, { kind: 'offer', offer_price: price, offer_status: 'pending' }),
    )
    save()
    return wait(job)
  },
  listJobs: () => {
    const u = me()
    return wait(db.jobs.filter((j) => j.customer.id === u.id || j.worker.id === u.id), 500)
  },
  getJob: (jobId) => {
    try {
      const job = findJob(jobId)
      return wait({ job, messages: db.messages.filter((m) => m.job_id === jobId) }, 300)
    } catch (e) { return Promise.reject(e) }
  },
  sendText: (jobId, body) => post(jobId, { body }),
  sendOffer: (jobId, offer_price) => post(jobId, { kind: 'offer', offer_price, offer_status: 'pending' }),
  sendVoice: (jobId) => post(jobId, { kind: 'voice', audio_url: null, body: null }),
  acceptOffer: (jobId, messageId) => {
    try {
      const job = findJob(jobId), m = db.messages.find((x) => x.id === messageId && x.job_id === jobId)
      if (!m || m.kind !== 'offer' || m.offer_status !== 'pending') throw new ApiError(400, 'offer not pending')
      if (m.sender_id === me().id) throw new ApiError(403, 'only the other party can accept')
      const busy = db.jobs.some((j) => j.id !== job.id && j.worker.id === job.worker.id && (j.status === 'confirmed' || j.status === 'in_progress'))
      if (busy) throw new ApiError(409, 'worker_busy')
      m.offer_status = 'accepted'
      Object.assign(job, { status: 'confirmed', agreed_price: m.offer_price })
      refreshWorker(job)
      save()
      return wait(job)
    } catch (e) { return Promise.reject(e) }
  },
  rejectOffer: (jobId, messageId) => {
    try {
      findJob(jobId)
      const m = db.messages.find((x) => x.id === messageId && x.job_id === jobId)
      if (!m || m.offer_status !== 'pending' || m.sender_id === me().id) throw new ApiError(400, 'offer not pending')
      m.offer_status = 'rejected'
      save()
      return wait(m)
    } catch (e) { return Promise.reject(e) }
  },
  setStatus: (jobId, status) => {
    try {
      const job = findJob(jobId), u = me()
      if (status !== 'cancelled' && u.role !== 'worker') throw new ApiError(403, 'only the worker starts or completes a job')
      if (job.status === 'completed' || job.status === 'cancelled') throw new ApiError(400, 'job closed')
      job.status = status
      if (status === 'completed') {
        job.completed_at = now()
        db.workers.find((x) => x.id === job.worker.id)!.jobs_done++
      }
      refreshWorker(job)
      save()
      return wait(job)
    } catch (e) { return Promise.reject(e) }
  },
  review: (jobId, r) => {
    try {
      const job = findJob(jobId)
      if (job.customer.id !== me().id || job.status !== 'completed' || job.reviewed) throw new ApiError(403, 'cannot review this job')
      const review: Review = { job_id: jobId, rating: r.rating, body: r.body || null, audio_url: null, tags: [], created_at: now() }
      job.reviewed = true
      db.workers.find((x) => x.id === job.worker.id)!.reviews.unshift(review)
      save()
      return wait(review)
    } catch (e) { return Promise.reject(e) }
  },
}

function mySession(): Promise<Me> {
  try {
    const user = me()
    const d = db.workers.find((x) => x.id === workerUser.id)!
    return wait({ user, worker: user.role === 'worker' && db.onboarded ? card(d) : null }, 300)
  } catch (e) { return Promise.reject(e) }
}

function post(jobId: number, m: Partial<Message>): Promise<Message> {
  try {
    const job = findJob(jobId)
    if (job.status === 'completed' || job.status === 'cancelled') throw new ApiError(400, 'job closed')
    const msg = message(jobId, me().id, m)
    db.messages.push(msg)
    save()
    return wait(msg, 300)
  } catch (e) { return Promise.reject(e) }
}

/** Demo stand-in when the browser blocks location (G-10, Islamabad). */
export const MOCK_LOCATION = { lat: 33.6762, lng: 73.014 }

/** The worker account used when signing in as a worker in demo mode. */
export const MOCK_WORKER_ID = workerUser.id

/** Sign out of the mock session. */
export function mockSignOut() {
  db.role = null
  save()
}
