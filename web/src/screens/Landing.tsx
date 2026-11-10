import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { Icon, type IconName } from '../components/Icon'
import { problemDraft } from '../lib/draft'
import { useLang, useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { TRADE_KEYS, TRADES } from '../lib/trades'

type Pair = [string, string] // [English, Urdu]

/** Elements marked data-reveal fade and rise into place the first time they scroll into view. */
function useReveal() {
  useEffect(() => {
    const els = document.querySelectorAll<HTMLElement>('.sp [data-reveal]')
    const io = new IntersectionObserver((entries) => entries.forEach((e) => {
      if (e.isIntersecting) { e.target.classList.add('in'); io.unobserve(e.target) }
    }), { threshold: 0.12 })
    els.forEach((el) => io.observe(el))
    return () => io.disconnect()
  }, [])
}

/** A number that counts up once, when it first comes into view. */
function Count({ to }: { to: number }) {
  const ref = useRef<HTMLSpanElement>(null)
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches
  const [n, setN] = useState(still ? to : 0)
  useEffect(() => {
    if (still) return
    let timer: ReturnType<typeof setInterval> | undefined
    const io = new IntersectionObserver(([e]) => {
      if (!e.isIntersecting) return
      io.disconnect()
      let step = 0
      timer = setInterval(() => {
        step++
        setN(Math.round(to * (1 - Math.pow(1 - step / 24, 3))))
        if (step >= 24) clearInterval(timer)
      }, 35)
    })
    if (ref.current) io.observe(ref.current)
    return () => { io.disconnect(); clearInterval(timer) }
  }, [to, still])
  return <span ref={ref} className="digits">{n}</span>
}

/** The statement lights up word by word as the section scrolls past. */
function Statement({ dim, bright }: { dim: string; bright: string }) {
  const ref = useRef<HTMLElement>(null)
  const [lit, setLit] = useState(0)
  const words = [...dim.split(' ').map((w) => ({ w, hi: false })), ...bright.split(' ').map((w) => ({ w, hi: true }))]
  useEffect(() => {
    const onScroll = () => {
      const el = ref.current
      if (!el) return
      const r = el.getBoundingClientRect()
      const progress = Math.min(1, Math.max(0, (innerHeight * 0.75 - r.top) / (r.height * 0.75)))
      setLit(Math.round(progress * words.length))
    }
    onScroll()
    addEventListener('scroll', onScroll, { passive: true })
    return () => removeEventListener('scroll', onScroll)
  }, [words.length])
  return (
    <section className="sp-dark" ref={ref}>
      <div className="sp-chips" aria-hidden="true">
        <span className="sp-chip" style={{ '--x': '6%', '--y': '14%', '--d': '0s' } as CSSProperties}><Icon name="MapPin" />1.2 KM</span>
        <span className="sp-chip" style={{ '--x': '74%', '--y': '10%', '--d': '.8s' } as CSSProperties}><Icon name="Clock" />FREE NOW</span>
        <span className="sp-chip" style={{ '--x': '80%', '--y': '70%', '--d': '1.6s' } as CSSProperties}><Icon name="Banknote" />RS 1,250–2,300</span>
        <span className="sp-chip" style={{ '--x': '4%', '--y': '74%', '--d': '2.2s' } as CSSProperties}><Icon name="Star" />4.7 · 3 JOBS</span>
        <span className="sp-chip" style={{ '--x': '42%', '--y': '86%', '--d': '1.2s' } as CSSProperties}><Icon name="Mic" />URDU VOICE</span>
      </div>
      <p className="sp-statement">
        {words.map((x, i) => <span key={i} className={`${i < lit ? 'lit' : ''} ${x.hi ? 'hi' : ''}`}>{x.w} </span>)}
      </p>
    </section>
  )
}

function Feature({ flip, icon, tag, title, text, cta, href, stat, statLabel, children }: {
  flip?: boolean; icon: IconName; tag: string; title: string; text: string; cta: string; href: string; stat: string; statLabel: string; children: ReactNode
}) {
  return (
    <section className={`sp-feature ${flip ? 'flip' : ''}`}>
      <div className="sp-feature-copy" data-reveal>
        <p className="sp-tag"><span className="sp-tag-icon"><Icon name={icon} /></span>{tag}</p>
        <h2>{title}</h2>
        <p className="sp-body">{text}</p>
        <Link className="sp-btn ghost" href={href}>{cta}</Link>
        <p className="sp-stat"><strong className="digits">{stat}</strong>{statLabel}</p>
      </div>
      <div className="sp-feature-visual" data-reveal>{children}</div>
    </section>
  )
}

export function Landing() {
  const t = useT()
  const ur = useLang() === 'ur'
  const forward: IconName = ur ? 'ArrowLeft' : 'ArrowRight'
  useReveal()

  const steps: { tag: Pair; title: Pair; text: Pair; icon: IconName }[] = [
    { icon: 'Mic', tag: ['Customer', 'گاہک'], title: ['Describe the problem', 'مسئلہ بتائیں'], text: ['Type or speak. The right trade and a fair price range come back in seconds.', 'لکھیں یا بولیں۔ چند سیکنڈ میں صحیح ہنر اور مناسب قیمت سامنے آ جاتی ہے۔'] },
    { icon: 'MapPin', tag: ['Customer', 'گاہک'], title: ['Pick a worker who is free', 'فارغ کاریگر چنیں'], text: ['Only workers who can come now are listed, nearest first, with ratings from real jobs.', 'صرف وہی کاریگر دکھتے ہیں جو ابھی آ سکتے ہیں، قریب والا پہلے، اصل کاموں کی ریٹنگ کے ساتھ۔'] },
    { icon: 'Banknote', tag: ['Both', 'دونوں'], title: ['Agree the price in chat', 'چیٹ میں قیمت طے کریں'], text: ['Either side sends a price. The job is booked only when the other accepts.', 'کوئی بھی قیمت بھیج سکتا ہے۔ کام تب پکا ہوتا ہے جب دوسرا منظور کرے۔'] },
    { icon: 'Wrench', tag: ['Worker', 'کاریگر'], title: ['Do the work', 'کام کریں'], text: ['The worker starts and completes the job with one big button each.', 'کاریگر ایک ایک بڑے بٹن سے کام شروع اور مکمل کرتا ہے۔'] },
    { icon: 'Star', tag: ['Customer', 'گاہک'], title: ['Leave a review', 'ریویو دیں'], text: ['Stars, text or a voice note. It helps the next neighbour choose.', 'ستارے، تحریر یا آواز۔ اگلے پڑوسی کو چننے میں مدد ملتی ہے۔'] },
  ]

  return (
    <main id="main-content" className="sp">
      <Link className="sp-news" href="/worker/login">
        <span className="sp-news-tag">{t('NEW', 'نیا')}</span>
        {t('Workers can now join by voice, in Urdu', 'کاریگر اب اردو میں بول کر شامل ہو سکتے ہیں')}
        <Icon name={forward} className="icon-sm" />
      </Link>

      <section className="sp-hero">
        <div className="sp-ruler left" aria-hidden="true" /><div className="sp-ruler right" aria-hidden="true" />
        <h1 data-reveal>{t('Skilled hands,', 'ماہر کاریگر،')}<br />{t('a few streets away', 'چند گلیاں دور')}</h1>
        <div className="sp-stage" data-reveal>
          <div className="sp-card float a">
            <p className="sp-mono">{t('PROBLEM', 'مسئلہ')}</p>
            <p dir="auto">Kitchen ka nalka leak ho raha hai</p>
          </div>
          <div className="sp-card float b">
            <p className="sp-mono">{t('TRADE', 'ہنر')}</p>
            <p className="sp-row"><Icon name={TRADES.plumber.icon} />{t('Plumber', 'پلمبر')} <span className="sp-pill digits">0.98</span></p>
          </div>
          <img src="/craft-hero.png" width={1100} height={733}
            alt={t('Worn hand tools, a copper pipe and pressure gauges on a stone', 'پتھر پر رکھے پرانے اوزار، تانبے کا پائپ اور گیج')} />
          <div className="sp-card float c">
            <p className="sp-row"><span className="avatar">AR</span><span><strong lang="en" dir="ltr">Abdul Rehman</strong><small><span className="digits">1.2 km</span> · {t('free now', 'ابھی فارغ')}</small></span></p>
          </div>
          <div className="sp-card float d">
            <p className="sp-mono">{t('AGREED PRICE', 'طے شدہ قیمت')}</p>
            <p className="sp-price digits">Rs 1,500 <Icon name="CircleCheck" /></p>
          </div>
        </div>
        <p className="sp-lede" data-reveal>{t(
          'Roshan Rozgar takes a household problem, in your own words, and turns it into a booked job with a nearby worker and a price you both agreed.',
          'روشن روزگار آپ کے اپنے الفاظ میں بتائے گئے مسئلے کو قریب کے کاریگر کے ساتھ پکے کام میں بدل دیتا ہے، ایسی قیمت پر جو دونوں نے طے کی ہو۔',
        )}</p>
        <div className="sp-actions" data-reveal>
          <Link className="sp-btn" href="/app/find">{t('I need a worker', 'مجھے کاریگر چاہیے')}<Icon name={forward} /></Link>
          <Link className="sp-btn ghost" href="/worker/login">{t('I am a worker', 'میں کاریگر ہوں')}</Link>
        </div>
      </section>

      <section className="sp-strip" id="services">
        <p className="sp-mono center">{t('Five trades across Islamabad & Rawalpindi', 'اسلام آباد اور راولپنڈی میں پانچ ہنر')}</p>
        <div className="sp-trades">
          {TRADE_KEYS.map((k, i) => (
            <Link key={k} className="sp-trade" href="/app/find/confirm" data-reveal style={{ transitionDelay: `${i * 70}ms` }}
              onClick={() => problemDraft.set({ trade: k, trade_confidence: 1, description: '', price_range: null })}>
              <Icon name={TRADES[k].icon} /><strong>{t(TRADES[k].en, TRADES[k].ur)}</strong><small>{t(TRADES[k].blurb, TRADES[k].blurbUr)}</small>
            </Link>
          ))}
        </div>
      </section>

      <Statement
        dim={t('Finding a good worker still runs on phone calls and guesswork.', 'اچھا کاریگر ڈھونڈنا آج بھی فون کالوں اور اندازوں پر چلتا ہے۔')}
        bright={t('We show who is free, how far, and for how much.', 'ہم دکھاتے ہیں کون فارغ ہے، کتنا دور ہے، اور کتنے میں۔')}
      />

      <Feature icon="House" tag={t('FOR HOMES', 'گھروں کے لیے')} title={t('Describe it your way', 'اپنے انداز میں بتائیں')}
        text={t('Type or speak in Urdu, Roman Urdu or English. The problem is read, matched to the right trade, and priced against completed jobs.', 'اردو، رومن اردو یا انگریزی میں لکھیں یا بولیں۔ مسئلہ پڑھ کر صحیح ہنر سے جوڑا جاتا ہے اور مکمل کاموں کے حساب سے قیمت بتائی جاتی ہے۔')}
        cta={t('Describe your problem', 'اپنا مسئلہ بتائیں')} href="/app/find" stat="~2 s" statLabel={t('TO FIND THE RIGHT TRADE', 'میں صحیح ہنر مل جاتا ہے')}>
        <div className="sp-mock">
          <p className="sp-bubble" dir="auto">AC bilkul thandi hawa nahi de raha, aaj hi koi aa jaye</p>
          <div className="sp-mock-row">
            <span className="sp-pill"><Icon name={TRADES.ac_technician.icon} />{t(TRADES.ac_technician.en, TRADES.ac_technician.ur)}</span>
            <span className="sp-pill warn">{t('Today', 'آج')}</span>
          </div>
          <p className="sp-mono">{t('TYPICAL PRICE', 'عام قیمت')}</p>
          <p className="sp-price digits">Rs 1,500–3,500</p>
        </div>
      </Feature>

      <Feature flip icon="Wrench" tag={t('FOR WORKERS', 'کاریگروں کے لیے')} title={t('Join by voice, in Urdu', 'اردو میں بول کر شامل ہوں')}
        text={t('No forms and no typing. Four spoken questions make the profile, and every job and message can be heard aloud.', 'نہ فارم، نہ لکھنا۔ چار سوالوں کے جواب بول کر پروفائل بن جاتا ہے، اور ہر کام اور میسج سنا جا سکتا ہے۔')}
        cta={t('Join as a worker', 'کاریگر بن کر شامل ہوں')} href="/worker/login" stat="4" statLabel={t('SPOKEN QUESTIONS TO JOIN', 'سوال، بول کر جواب')}>
        <div className="sp-mock voice">
          <p className="sp-question" lang="ur" dir="rtl">آپ کیا کام کرتے ہیں؟</p>
          <span className="sp-mic"><Icon name="Mic" /></span>
          <p className="sp-heard"><Icon name="CircleCheck" /><span lang="ur" dir="rtl">ٹھیک ہے · پلمبر کا کام</span></p>
        </div>
      </Feature>

      <Feature icon="ShieldCheck" tag={t('BOOKING', 'بکنگ')} title={t('One worker, one job', 'ایک کاریگر، ایک کام')}
        text={t('The price is agreed before anyone travels, and a worker on a job cannot be booked by someone else. The rule lives in the database, not in good intentions.', 'قیمت پہلے طے ہوتی ہے، اور جو کاریگر کام پر ہو اسے کوئی اور بک نہیں کر سکتا۔ یہ اصول ڈیٹابیس میں لکھا ہے، صرف ارادوں میں نہیں۔')}
        cta={t('See how it works', 'طریقہ دیکھیں')} href="/#how-it-works" stat="1" statLabel={t('ACTIVE JOB PER WORKER, ENFORCED', 'وقت میں ایک ہی کام، لازمی')}>
        <div className="sp-mock">
          <p className="sp-mono">{t('PRICE OFFER', 'قیمت کی پیشکش')}</p>
          <p className="sp-price digits">Rs 1,500</p>
          <div className="sp-mock-row">
            <span className="sp-btn small"><Icon name="Check" />{t('Accept', 'منظور')}</span>
            <span className="sp-btn ghost small"><Icon name="X" />{t('Decline', 'نہیں')}</span>
          </div>
          <p className="sp-heard"><Icon name="CircleCheck" />{t('Job confirmed', 'کام پکا ہو گیا')}</p>
        </div>
      </Feature>

      <section className="sp-steps" id="how-it-works">
        <div className="sp-steps-head">
          <h2 data-reveal>{t('From problem to paid,', 'مسئلے سے ادائیگی تک،')}<br />{t('in five steps', 'پانچ قدموں میں')}</h2>
          <img src="/craft-hero.png" width={1100} height={733} alt="" loading="lazy" />
        </div>
        <ol>
          {steps.map((s, i) => (
            <li key={s.title[0]} data-reveal>
              <span className="sp-step-dot"><Icon name={s.icon} /></span>
              <div>
                <p className="sp-mono">[{t(...s.tag)}] · <span className="digits">0{i + 1}</span></p>
                <h3>{t(...s.title)}</h3>
                <p className="sp-body">{t(...s.text)}</p>
              </div>
            </li>
          ))}
        </ol>
      </section>

      <section className="sp-sage">
        <h2 data-reveal>{t('Built to be fair to both sides', 'دونوں کے لیے منصفانہ')}</h2>
        <div className="sp-numbers">
          {([[5, ['TRADES COVERED', 'ہنر']], [4, ['SPOKEN QUESTIONS TO JOIN', 'سوال، بول کر شامل ہوں']], [2, ['LANGUAGES, ONE SWITCH', 'زبانیں، ایک بٹن']], [1, ['JOB AT A TIME PER WORKER', 'وقت میں ایک کام']]] as [number, Pair][]).map(([n, label]) => (
            <div key={label[0]} data-reveal><strong><Count to={n} /></strong><p className="sp-mono">{t(...label)}</p></div>
          ))}
        </div>
      </section>

      <section className="sp-cta">
        <h2 data-reveal>{t('Get it fixed today', 'آج ہی کام کروائیں')}</h2>
        <div className="sp-actions" data-reveal>
          <Link className="sp-btn lime" href="/app/find">{t('Describe your problem', 'اپنا مسئلہ بتائیں')}<Icon name={forward} /></Link>
          <Link className="sp-btn ghost on-dark" href="/worker/login">{t('Join as a worker', 'کاریگر بن کر شامل ہوں')}</Link>
        </div>
      </section>
    </main>
  )
}
