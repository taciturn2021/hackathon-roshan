-- Source of truth: docs/5-BACKEND_SCHEMA.md. Change both in the same PR.
CREATE TABLE users (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  firebase_uid TEXT NOT NULL UNIQUE,   -- Firebase Auth user id; seeded users use 'seed-<n>'
  email      TEXT,
  phone      TEXT,                    -- E.164, from Firebase phone sign-in
  role       TEXT NOT NULL CHECK (role IN ('worker','customer')),
  name       TEXT,                    -- prefilled from the Google account
  photo_url  TEXT,                    -- prefilled from the Google account
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE worker_profiles (
  user_id         BIGINT PRIMARY KEY REFERENCES users(id),
  trade           TEXT NOT NULL CHECK (trade IN ('electrician','plumber','ac_technician','carpenter','mechanic')),
  skills          TEXT[] NOT NULL DEFAULT '{}',
  years_exp       INT,
  area            TEXT,
  lat             DOUBLE PRECISION NOT NULL,
  lng             DOUBLE PRECISION NOT NULL,
  photo_url       TEXT,
  intro_audio_url TEXT,
  is_online       BOOLEAN NOT NULL DEFAULT false,
  trust_summary   TEXT
);
CREATE INDEX ON worker_profiles (trade) WHERE is_online;

CREATE TABLE jobs (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  customer_id  BIGINT NOT NULL REFERENCES users(id),
  worker_id    BIGINT NOT NULL REFERENCES users(id),
  trade        TEXT NOT NULL,
  description  TEXT NOT NULL,
  urgency      TEXT NOT NULL CHECK (urgency IN ('now','today','flexible')),
  photo_url    TEXT,
  lat          DOUBLE PRECISION NOT NULL,
  lng          DOUBLE PRECISION NOT NULL,
  status       TEXT NOT NULL DEFAULT 'requested'
               CHECK (status IN ('requested','confirmed','in_progress','completed','cancelled')),
  agreed_price INT,
  brief_ur        TEXT,          -- short Urdu summary for the worker, filled by the queue
  brief_audio_url TEXT,          -- its audio, played when the worker taps the speaker
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);
-- The booking guarantee: a worker can hold at most one active job.
CREATE UNIQUE INDEX one_active_job_per_worker
  ON jobs (worker_id) WHERE status IN ('confirmed','in_progress');
CREATE INDEX ON jobs (customer_id);

CREATE TABLE messages (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  job_id          BIGINT NOT NULL REFERENCES jobs(id),
  sender_id       BIGINT NOT NULL REFERENCES users(id),
  kind            TEXT NOT NULL CHECK (kind IN ('text','voice','offer')),
  body            TEXT,          -- text, or transcript once the queue fills it
  audio_url       TEXT,
  body_ur         TEXT,          -- typed text rewritten in Urdu script, filled by the queue
  speech_url      TEXT,          -- its audio, so the worker can listen to a typed message
  suggested_price INT,           -- price heard in a voice note; sender taps to turn it into an offer
  offer_price     INT,
  offer_status    TEXT CHECK (offer_status IN ('pending','accepted','rejected')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON messages (job_id, id);

CREATE TABLE reviews (
  job_id     BIGINT PRIMARY KEY REFERENCES jobs(id),   -- one review per job
  rating     INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  body       TEXT,
  audio_url  TEXT,
  tags       TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Availability is derived, never stored.
CREATE VIEW available_workers AS
  SELECT w.* FROM worker_profiles w
  WHERE w.is_online
    AND NOT EXISTS (SELECT 1 FROM jobs j
                    WHERE j.worker_id = w.user_id AND j.status IN ('confirmed','in_progress'));
