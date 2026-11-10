-- Brings a database created from an older schema.sql up to date. Idempotent: safe to run again.
-- Fresh databases get these columns from schema.sql directly.
--   docker exec -i hackathon-postgres-1 psql -U roshan -d roshan < backend/db/migrations/002_phone_and_urdu_speech.sql
ALTER TABLE users    ADD COLUMN IF NOT EXISTS phone      TEXT;   -- Firebase phone sign-in
ALTER TABLE jobs     ADD COLUMN IF NOT EXISTS brief_ur   TEXT;   -- short Urdu summary for the worker
ALTER TABLE messages ADD COLUMN IF NOT EXISTS body_ur    TEXT;   -- typed text in Urdu script
ALTER TABLE messages ADD COLUMN IF NOT EXISTS speech_url TEXT;   -- its audio
