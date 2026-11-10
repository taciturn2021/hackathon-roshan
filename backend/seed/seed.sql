-- Roshan Rozgar demo seed. Idempotent: safe to run twice (no errors, no duplicates).
-- Load after backend/db/schema.sql. Deterministic: no random(), same data everywhere.
-- Workers: firebase_uid 'seed-1'..'seed-40' (8 per trade, in trade order).
-- Customers: firebase_uid 'seed-c1'..'seed-c8'.
-- Jobs are only 'completed' or 'cancelled' so nobody is made unavailable.
BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Worker definitions (temp table, dropped at COMMIT)
-- ---------------------------------------------------------------------------
CREATE TEMP TABLE seed_workers (
  n INT, name TEXT, trade TEXT, skills TEXT, years_exp INT, area TEXT
) ON COMMIT DROP;

INSERT INTO seed_workers (n, name, trade, skills, years_exp, area) VALUES
  -- electricians
  (1,  'Muhammad Akram',   'electrician', '{wiring,short circuit,switchboards}',        14, 'G-10'),
  (2,  'Tariq Mehmood',    'electrician', '{fans,lighting,wiring}',                      8, 'G-9'),
  (3,  'Imran Shahzad',    'electrician', '{ups,inverter,breaker repair}',              11, 'G-11'),
  (4,  'Rashid Ali',       'electrician', '{wiring,fans,short circuit,lighting}',       19, 'F-10'),
  (5,  'Zeeshan Haider',   'electrician', '{switchboards,lighting}',                     4, 'F-7'),
  (6,  'Naveed Iqbal',     'electrician', '{wiring,breaker repair}',                    23, 'I-8'),
  (7,  'Shakeel Ahmed',    'electrician', '{fans,ups,short circuit}',                    6, 'Saddar, Rawalpindi'),
  (8,  'Waseem Abbas',     'electrician', '{wiring,lighting,fans}',                      2, 'Bahria Town, Rawalpindi'),
  -- plumbers
  (9,  'Abdul Rehman',     'plumber',     '{leaks,geyser,drainage,fittings}',           12, 'G-10'),
  (10, 'Khalid Hussain',   'plumber',     '{drainage,tank cleaning}',                   17, 'G-9'),
  (11, 'Sajid Nawaz',      'plumber',     '{leaks,fittings,bathroom fitting}',           7, 'F-10'),
  (12, 'Farooq Ahmad',     'plumber',     '{geyser,leaks}',                              9, 'G-11'),
  (13, 'Liaqat Ali',       'plumber',     '{drainage,fittings,leaks}',                  25, 'F-6'),
  (14, 'Mushtaq Hussain',  'plumber',     '{bathroom fitting,fittings}',                 5, 'E-11'),
  (15, 'Adnan Qureshi',    'plumber',     '{leaks,geyser,tank cleaning}',               10, 'Satellite Town, Rawalpindi'),
  (16, 'Babar Khan',       'plumber',     '{drainage,leaks}',                            3, 'I-10'),
  -- ac technicians
  (17, 'Faisal Mahmood',   'ac_technician', '{ac repair,gas refill,servicing}',         13, 'G-11'),
  (18, 'Shahid Pervaiz',   'ac_technician', '{installation,servicing}',                  8, 'G-10'),
  (19, 'Arshad Javed',     'ac_technician', '{gas refill,compressor,ac repair,servicing}', 18, 'G-9'),
  (20, 'Danish Raza',      'ac_technician', '{installation,ac repair}',                  3, 'F-10'),
  (21, 'Nadeem Akhtar',    'ac_technician', '{servicing,gas refill}',                    6, 'Blue Area'),
  (22, 'Hassan Raza',      'ac_technician', '{compressor,ac repair,installation}',      15, 'F-8'),
  (23, 'Ghulam Mustafa',   'ac_technician', '{ac repair,gas refill,servicing}',         21, 'Saddar, Rawalpindi'),
  (24, 'Owais Qarni',      'ac_technician', '{installation,servicing}',                  4, 'I-8'),
  -- carpenters
  (25, 'Ustad Rafiq',      'carpenter',   '{furniture,doors,polish}',                   24, 'G-10'),
  (26, 'Yasir Mahmood',    'carpenter',   '{kitchen cabinets,wardrobes}',                9, 'G-11'),
  (27, 'Zahid Hussain',    'carpenter',   '{doors,furniture repair,polish}',            16, 'F-10'),
  (28, 'Anwar ul Haq',     'carpenter',   '{wardrobes,furniture,doors}',                12, 'G-9'),
  (29, 'Shoaib Akhtar',    'carpenter',   '{kitchen cabinets,polish}',                   5, 'I-10'),
  (30, 'Mansoor Elahi',    'carpenter',   '{furniture,wardrobes,doors,polish}',         20, 'Satellite Town, Rawalpindi'),
  (31, 'Qasim Ali',        'carpenter',   '{doors,furniture repair}',                    7, 'Bahria Town, Rawalpindi'),
  (32, 'Irfan Baig',       'carpenter',   '{wardrobes,kitchen cabinets}',                3, 'F-7'),
  -- mechanics
  (33, 'Altaf Hussain',    'mechanic',    '{engine,oil change,brakes}',                 22, 'G-10'),
  (34, 'Junaid Ashraf',    'mechanic',    '{brakes,suspension,battery}',                10, 'G-9'),
  (35, 'Kashif Mehmood',   'mechanic',    '{engine,car ac,tuning}',                     13, 'G-11'),
  (36, 'Rizwan Saleem',    'mechanic',    '{oil change,battery}',                        2, 'F-10'),
  (37, 'Sarfraz Ahmed',    'mechanic',    '{engine,suspension,brakes,oil change}',      18, 'E-11'),
  (38, 'Tahir Mehboob',    'mechanic',    '{brakes,battery,tuning}',                     6, 'Saddar, Rawalpindi'),
  (39, 'Amjad Pervez',     'mechanic',    '{engine,car ac}',                            15, 'I-8'),
  (40, 'Noman Siddique',   'mechanic',    '{suspension,oil change,brakes}',              8, 'Satellite Town, Rawalpindi');

-- ---------------------------------------------------------------------------
-- 2. Users: 40 workers + 8 customers
-- ---------------------------------------------------------------------------
INSERT INTO users (firebase_uid, email, phone, role, name, photo_url)
SELECT 'seed-' || n, NULL,
       CASE WHEN n % 9 = 0 THEN NULL
            ELSE '+92300' || lpad(((n * 1234567) % 10000000)::text, 7, '0') END,
       'worker', name, NULL
FROM seed_workers
ON CONFLICT (firebase_uid) DO NOTHING;

INSERT INTO users (firebase_uid, email, phone, role, name, photo_url) VALUES
  ('seed-c1', NULL, '+923211100201', 'customer', 'Ayesha Siddiqui', NULL),
  ('seed-c2', NULL, '+923331100202', 'customer', 'Bilal Ahmed',     NULL),
  ('seed-c3', NULL, NULL,            'customer', 'Sana Malik',       NULL),
  ('seed-c4', NULL, '+923451100204', 'customer', 'Usman Tariq',     NULL),
  ('seed-c5', NULL, '+923011100205', 'customer', 'Hamza Sheikh',    NULL),
  ('seed-c6', NULL, NULL,            'customer', 'Fatima Noor',     NULL),
  ('seed-c7', NULL, '+923151100207', 'customer', 'Zain Abbas',      NULL),
  ('seed-c8', NULL, '+923401100208', 'customer', 'Kamran Butt',     NULL)
ON CONFLICT (firebase_uid) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 3. Worker profiles. Area centres + small deterministic jitter (+-0.006).
--    34 of 40 online (offline: 7, 14, 21, 28, 35, 40).
-- ---------------------------------------------------------------------------
INSERT INTO worker_profiles (user_id, trade, skills, years_exp, area, lat, lng, is_online)
SELECT u.id, w.trade, w.skills::text[], w.years_exp, w.area,
       a.lat + (((w.n * 37) % 13) - 6) / 1000.0,
       a.lng + (((w.n * 53) % 13) - 6) / 1000.0,
       (w.n % 7 <> 0 AND w.n <> 40)
FROM seed_workers w
JOIN users u ON u.firebase_uid = 'seed-' || w.n
JOIN (VALUES
  ('F-6', 33.7294, 73.0758), ('F-7', 33.7215, 73.0560), ('F-8', 33.7100, 73.0400),
  ('F-10', 33.6950, 73.0150), ('G-9', 33.6900, 73.0300), ('G-10', 33.6844, 73.0145),
  ('G-11', 33.6700, 72.9950), ('I-8', 33.6680, 73.0750), ('I-10', 33.6480, 73.0380),
  ('E-11', 33.6990, 72.9750), ('Blue Area', 33.7100, 73.0600),
  ('Saddar, Rawalpindi', 33.5973, 73.0479), ('Satellite Town, Rawalpindi', 33.6380, 73.0650),
  ('Bahria Town, Rawalpindi', 33.5200, 73.1000)
) AS a(area, lat, lng) ON a.area = w.area
ON CONFLICT (user_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 4. Jobs (90 completed + 5 cancelled) and 5. reviews (one per completed job)
--    Workers 8, 20, 36 get no jobs/reviews. Skipped entirely if seed jobs exist.
-- ---------------------------------------------------------------------------
WITH
pool AS (
  SELECT array_agg(n ORDER BY n) AS a FROM generate_series(1, 40) n WHERE n NOT IN (8, 20, 36)
),
tp(trade, lo, hi, descs) AS (VALUES
  ('electrician', 800, 3500, ARRAY[
    'Bijli ka switchboard spark kar raha hai',
    'Fan not working, capacitor change karna hai',
    'Short circuit ho gaya, bijli chali gayi poore ghar ki',
    'Need 4 new light points in the lounge',
    'UPS ki wiring check karwani hai',
    'Breaker baar baar trip ho raha hai',
    'Bedroom mein new socket lagwana hai',
    'Geyser ki wiring jal gayi, please check']::text[]),
  ('plumber', 700, 3000, ARRAY[
    'Kitchen sink leak ho raha hai',
    'Bathroom drain blocked, pani ruk jata hai',
    'Geyser not heating, please check',
    'Washroom mein new commode fitting karni hai',
    'Water tank overflow ho raha hai',
    'Tap se pani tapak raha hai, replace karna hai',
    'Pipe burst in the lawn, urgent',
    'Basin ki fitting dheeli ho gayi hai']::text[]),
  ('ac_technician', 1500, 6000, ARRAY[
    'AC not cooling, gas check karna hai',
    '1.5 ton split AC ki installation karwani hai',
    'AC se pani tapak raha hai',
    'Full AC servicing before summer',
    'Outdoor unit se bohat awaz aa rahi hai',
    'Inverter AC ka compressor start nahi ho raha',
    'AC remote kaam nahi kar raha aur display off hai',
    'Need gas refill for 2 ton AC']::text[]),
  ('carpenter', 1000, 5000, ARRAY[
    'Darwaze ka lock aur hinge change karna hai',
    'Need a wardrobe built for the bedroom',
    'Kitchen cabinets ki repair karwani hai',
    'Sofa ki frame toot gayi hai',
    'Old dining table needs polish',
    'Window ka frame sadh gaya hai, replace karna hai',
    'Bed ka box fit karna hai aur drawer theek karna hai',
    'Wooden shelf lagwani hai drawing room mein']::text[]),
  ('mechanic', 1000, 8000, ARRAY[
    'Gaari start nahi ho rahi, battery check karein',
    'Brake pads need replacing',
    'Engine se awaz aa rahi hai, please check at home',
    'Oil change and filter, Corolla 2015',
    'Suspension se thak thak ki awaz aati hai',
    'Car AC not cooling, gas aur compressor dekhna hai',
    'Gaari ka tel lead kar raha hai',
    'Self starter kharab ho gaya hai']::text[])
),
rv(kind, idx, body, tags) AS (VALUES
  ('pos', 0,  'Kaam bohat acha kiya, time par pohanch gaye.',               ARRAY['on_time']),
  ('pos', 1,  'Very clean work, left no mess behind.',                      ARRAY['clean_work']),
  ('pos', 2,  'Rate bilkul theek tha, koi extra charge nahi.',              ARRAY['fair_price']),
  ('pos', 3,  'Polite and professional, explained the problem clearly.',    ARRAY['polite']),
  ('pos', 4,  'Time pe aaye aur kaam saaf suthra kiya. Recommended!',       ARRAY['on_time','clean_work']),
  ('pos', 5,  'Fair price and quick work. Will call again.',                ARRAY['fair_price','on_time']),
  ('pos', 6,  NULL,                                                         ARRAY['polite']),
  ('pos', 7,  'Bohat shareef aur mehnati banda hai, price bhi waajib.',     ARRAY['polite','fair_price']),
  ('pos', 8,  'Arrived within 20 minutes and fixed it on the spot.',        ARRAY['on_time']),
  ('pos', 9,  'Shukriya bhai, masla hal ho gaya. Kaam clean tha.',          ARRAY['clean_work']),
  ('pos', 10, 'Great experience, honest about the cost.',                   ARRAY['fair_price','polite']),
  ('pos', 11, NULL,                                                         ARRAY[]::text[]),
  ('pos', 12, 'Mashallah behtareen kaam, har cheez tarteeb se ki.',         ARRAY['clean_work','polite']),
  ('pos', 13, 'Reasonable rates, on time and respectful.',                  ARRAY['fair_price','on_time','polite']),
  ('mid', 0,  'Kaam theek tha lekin thora late aaye.',                      ARRAY[]::text[]),
  ('mid', 1,  'Work was okay but the price was a bit high.',                ARRAY['polite']),
  ('mid', 2,  'Average kaam, dobara check karwana para.',                   ARRAY[]::text[]),
  ('mid', 3,  'Decent job, par safai ka khayal nahi rakha.',                ARRAY['polite']),
  ('low', 0,  'Bohat late aaye aur kaam adhoora chor diya.',                ARRAY[]::text[]),
  ('low', 1,  'Price pehle se zyada manga, kaam bhi kharab. Not happy.',    ARRAY[]::text[])
),
gen AS (
  SELECT i,
    u.id AS worker_id, c.id AS customer_id, wp.trade,
    tp.descs[(i * 5) % 8 + 1] AS description,
    (ARRAY['now','today','flexible'])[(i * 2) % 3 + 1] AS urgency,
    wp.lat + (((i * 29) % 11) - 5) / 2000.0 AS lat,
    wp.lng + (((i * 31) % 11) - 5) / 2000.0 AS lng,
    CASE WHEN i <= 90 THEN 'completed' ELSE 'cancelled' END AS status,
    CASE WHEN i <= 90 THEN (tp.lo + (i * 263) % (tp.hi - tp.lo + 1)) / 50 * 50 END AS agreed_price,
    now() - (((i * 907) % 85000) + 900) * interval '1 minute' AS created_at,
    CASE WHEN i <= 90
      THEN now() - (((i * 907) % 85000) + 900) * interval '1 minute'
           + (2 + i % 5) * interval '1 hour' + (i % 4) * interval '15 minutes' END AS completed_at,
    CASE WHEN i IN (17, 71) THEN 'low' WHEN i % 15 = 4 THEN 'mid' ELSE 'pos' END AS rkind,
    CASE WHEN i IN (17, 71) THEN (CASE WHEN i = 17 THEN 0 ELSE 1 END)
         WHEN i % 15 = 4 THEN (i / 15) % 4
         ELSE (i * 3) % 14 END AS ridx,
    CASE WHEN i IN (17, 71) THEN 1
         WHEN i % 15 = 4 THEN 2 + (i / 15) % 2
         WHEN i % 4 = 0 THEN 4 ELSE 5 END AS rating
  FROM generate_series(1, 95) i
  CROSS JOIN pool
  JOIN users u ON u.firebase_uid = 'seed-' || pool.a[(i - 1) % 37 + 1]
  JOIN worker_profiles wp ON wp.user_id = u.id
  JOIN tp ON tp.trade = wp.trade
  JOIN users c ON c.firebase_uid = 'seed-c' || (i % 8 + 1)
  WHERE NOT EXISTS (
    SELECT 1 FROM jobs j JOIN users cu ON cu.id = j.customer_id
    WHERE cu.firebase_uid LIKE 'seed-%'
  )
),
ins AS (
  INSERT INTO jobs (customer_id, worker_id, trade, description, urgency, lat, lng,
                    status, agreed_price, created_at, completed_at)
  SELECT customer_id, worker_id, trade, description, urgency, lat, lng,
         status, agreed_price, created_at, completed_at
  FROM gen
  ORDER BY i
  RETURNING id, worker_id, created_at, completed_at, status
)
INSERT INTO reviews (job_id, rating, body, audio_url, tags, created_at)
SELECT ins.id, g.rating, rv.body, NULL, rv.tags,
       ins.completed_at + (1 + g.i % 3) * interval '1 hour'
FROM ins
JOIN gen g ON g.worker_id = ins.worker_id AND g.created_at = ins.created_at
JOIN rv ON rv.kind = g.rkind AND rv.idx = g.ridx
WHERE ins.status = 'completed'
ON CONFLICT (job_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 6. Trust summaries, derived from each seed worker's reviews (same result on re-run)
-- ---------------------------------------------------------------------------
WITH s AS (
  SELECT j.worker_id,
         avg(r.rating) AS avg_rating,
         count(*) FILTER (WHERE 'on_time'    = ANY(r.tags)) AS ot,
         count(*) FILTER (WHERE 'fair_price' = ANY(r.tags)) AS fp,
         count(*) FILTER (WHERE 'clean_work' = ANY(r.tags)) AS cw,
         count(*) FILTER (WHERE 'polite'     = ANY(r.tags)) AS po
  FROM jobs j
  JOIN reviews r ON r.job_id = j.id
  JOIN users u ON u.id = j.worker_id
  WHERE u.firebase_uid LIKE 'seed-%'
  GROUP BY j.worker_id
)
UPDATE worker_profiles w
SET trust_summary =
  (CASE WHEN s.avg_rating >= 4.5 THEN 'Highly rated by customers'
        WHEN s.avg_rating >= 3.8 THEN 'Generally well reviewed'
        ELSE 'Mixed reviews' END)
  || CASE
       WHEN greatest(s.ot, s.fp, s.cw, s.po) = 0 THEN '.'
       WHEN s.ot = greatest(s.ot, s.fp, s.cw, s.po) THEN '; customers say he arrives on time.'
       WHEN s.fp = greatest(s.ot, s.fp, s.cw, s.po) THEN '; most customers call the price fair.'
       WHEN s.cw = greatest(s.ot, s.fp, s.cw, s.po) THEN '; known for clean, tidy work.'
       ELSE '; polite and respectful with customers.'
     END
FROM s
WHERE w.user_id = s.worker_id;

COMMIT;
