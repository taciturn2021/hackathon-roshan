# 5. Backend Schema — Roshan Rozgar

Part of the six planning documents in `docs/`. Give all six to your AI coding assistant before asking it to build.

```sql
CREATE TABLE users (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  firebase_uid TEXT NOT NULL UNIQUE,   -- Firebase Auth user id; seeded users use 'seed-<n>'
  email      TEXT,
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
  brief_audio_url TEXT,          -- Urdu audio summary for the worker, filled by the queue
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
```

**Ownership rules (enforced in each `Service`)**
- A user edits only their own profile; only the two parties of a job can read it or post to it.
- Only the *other* party can accept an offer. Only the worker moves `confirmed → in_progress → completed`.
- Only the job's customer can review, and only when status is `completed`.
