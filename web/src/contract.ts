// API contract. Source of truth: docs/2-TRD.md — change both in the same PR and tag Humza.

export type Trade = 'electrician' | 'plumber' | 'ac_technician' | 'carpenter' | 'mechanic';
export type Urgency = 'now' | 'today' | 'flexible';
export type JobStatus = 'requested' | 'confirmed' | 'in_progress' | 'completed' | 'cancelled';

export type User = { id: number; email: string | null; role: 'worker' | 'customer'; name: string | null; photo_url: string | null };

export type OnboardField = 'name' | 'trade' | 'area' | 'years_exp';
export type OnboardAnswer = {                // AI reading of one spoken answer; nothing is saved yet
  field: OnboardField;                // the question that was asked
  profile: { name?: string; trade?: Trade; skills?: string[]; area?: string; years_exp?: number };
                                      // whatever was understood; may include other fields, empty if nothing
  confidence: number; transcript: string; audio_url: string;
};

export type WorkerCard = {
  id: number; name: string; trade: Trade; area: string | null; years_exp: number | null;
  photo_url: string | null; intro_audio_url: string | null;
  is_online: boolean; available: boolean; distance_km: number | null;
  rating_avg: number | null; jobs_done: number; trust_summary: string | null; tags: string[];
};
export type PriceRange = { low: number; high: number; sample: number } | null;
export type Review = { job_id: number; rating: number; body: string | null; audio_url: string | null; tags: string[]; created_at: string };
export type WorkerDetail = WorkerCard & { reviews: Review[]; price_range: PriceRange };

export type Problem = { trade: Trade | null; trade_confidence: number; urgency: Urgency;
                 description: string; photo_url: string | null; price_range: PriceRange };

export type Job = { id: number; customer: User; worker: WorkerCard; trade: Trade; description: string;
             urgency: Urgency; photo_url: string | null; status: JobStatus;
             agreed_price: number | null;
             brief_ur: string | null; brief_audio_url: string | null;   // short Urdu summary + its audio; arrive via job.updated
             created_at: string; completed_at: string | null; reviewed: boolean };
export type Message = { id: number; job_id: number; sender_id: number; kind: 'text' | 'voice' | 'offer';
                 body: string | null; audio_url: string | null;
                 body_ur: string | null; speech_url: string | null;   // typed text in Urdu script + its audio, for the worker
                 suggested_price: number | null;
                 offer_price: number | null; offer_status: 'pending' | 'accepted' | 'rejected' | null;
                 created_at: string };

export type Event =
  | { type: 'job.created';     data: Job }       // to the worker
  | { type: 'job.updated';     data: Job }       // to both parties
  | { type: 'message.created'; data: Message }   // to both parties
  | { type: 'message.updated'; data: Message };  // transcript, suggested_price, body_ur or speech_url filled in
