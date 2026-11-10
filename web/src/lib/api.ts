// REST client for the contract in docs/2-TRD.md. Every call goes to /api unless VITE_MOCK_API=true.
import type { Job, JobStatus, Message, OnboardAnswer, OnboardField, Problem, Review, Trade, Urgency, User, WorkerCard, WorkerDetail } from '../contract'
import { idToken } from './firebase'
import { mockApi } from './mocks'

export type Me = { user: User; worker: WorkerCard | null }
export type WorkerProfileInput = {
  name: string; trade: Trade; skills: string[]; years_exp: number | null; area: string | null
  lat: number | null; lng: number | null; intro_audio_url: string | null
}
export type NewJob = { worker_id: number; trade: Trade; description: string; urgency: Urgency; lat: number; lng: number; photo_url?: string }

export class ApiError extends Error {
  status: number
  /** The API's error code, e.g. `worker_busy`, `no_session`, `rate_limited`. */
  code?: string
  constructor(status: number, message: string, code?: string) {
    super(message)
    this.status = status
    this.code = code
  }
}
export const isWorkerBusy = (e: unknown) => e instanceof ApiError && (e.code ?? e.message) === 'worker_busy'

async function http<T>(method: string, path: string, body?: unknown): Promise<T> {
  const isForm = body instanceof FormData
  const token = await idToken()
  const res = await fetch('/api' + path, {
    method,
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(body && !isForm ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
  })
  if (!res.ok) {
    // The API answers errors as {"error": code, "message": text}.
    const e = (await res.json().catch(() => ({}))) as { error?: string; message?: string }
    throw new ApiError(res.status, e.message ?? res.statusText, e.error)
  }
  return res.json() as Promise<T>
}

const form = (fields: Record<string, string | Blob | undefined>) => {
  const f = new FormData()
  for (const [k, v] of Object.entries(fields)) if (v !== undefined) f.append(k, v)
  return f
}

const httpApi = {
  session: (role: User['role']) => http<Me>('POST', '/auth/session', { role }),
  me: () => http<Me>('GET', '/me'),
  onboard: (audio: Blob, field: OnboardField) => http<OnboardAnswer>('POST', '/workers/onboard', form({ audio, field })),
  saveWorker: (p: WorkerProfileInput) => http<WorkerCard>('PUT', '/workers/me', p),
  setOnline: (is_online: boolean) => http<WorkerCard>('PUT', '/workers/me/online', { is_online }),
  listWorkers: (q: { trade: Trade; lat: number; lng: number; radius_km: number }) =>
    http<WorkerCard[]>('GET', `/workers?trade=${q.trade}&lat=${q.lat}&lng=${q.lng}&radius_km=${q.radius_km}`),
  getWorker: (id: number) => http<WorkerDetail>('GET', `/workers/${id}`),
  analyzeProblem: (p: { text?: string; audio?: Blob }) => http<Problem>('POST', '/problems', form(p)),
  createJob: (j: NewJob) => http<Job>('POST', '/jobs', j),
  listJobs: () => http<Job[]>('GET', '/jobs'),
  getJob: (id: number) => http<{ job: Job; messages: Message[] }>('GET', `/jobs/${id}`),
  sendText: (jobId: number, body: string) => http<Message>('POST', `/jobs/${jobId}/messages`, { kind: 'text', body }),
  sendOffer: (jobId: number, offer_price: number) => http<Message>('POST', `/jobs/${jobId}/messages`, { kind: 'offer', offer_price }),
  sendVoice: (jobId: number, audio: Blob) => http<Message>('POST', `/jobs/${jobId}/messages`, form({ audio })),
  acceptOffer: (jobId: number, messageId: number) => http<Job>('POST', `/jobs/${jobId}/offers/${messageId}/accept`),
  rejectOffer: (jobId: number, messageId: number) => http<Message>('POST', `/jobs/${jobId}/offers/${messageId}/reject`),
  setStatus: (jobId: number, status: Exclude<JobStatus, 'requested' | 'confirmed'>) => http<Job>('POST', `/jobs/${jobId}/status`, { status }),
  review: (jobId: number, r: { rating: number; body?: string; audio?: Blob }) =>
    http<Review>('POST', `/jobs/${jobId}/review`, form({ rating: String(r.rating), body: r.body, audio: r.audio })),
}
export type Api = typeof httpApi

/** VITE_MOCK_API=true runs the screens on the in-browser mock backend (fictional data) instead of the API. */
export const MOCK = import.meta.env.VITE_MOCK_API === 'true'
export const api: Api = MOCK ? mockApi : httpApi
