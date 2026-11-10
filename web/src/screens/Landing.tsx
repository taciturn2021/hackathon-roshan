import { Icon } from '../components/Icon'
import { problemDraft } from '../lib/draft'
import { useLang, useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { TRADE_KEYS, TRADES } from '../lib/trades'

// [number, [title en, ur], [text en, ur]]
const steps: [string, [string, string], [string, string]][] = [
  ['01', ['Tell us what’s wrong', 'بتائیں کیا خراب ہے'],
    ['Type or speak in English, Urdu or Roman Urdu. We help you find the right trade.', 'انگریزی، اردو یا رومن اردو میں لکھیں یا بولیں۔ ہم صحیح کاریگر چننے میں مدد کرتے ہیں۔']],
  ['02', ['Agree a price in chat', 'چیٹ میں قیمت طے کریں'],
    ['Choose an available worker nearby. Discuss the job and accept a clear price offer.', 'قریب کا فارغ کاریگر چنیں۔ کام پر بات کریں اور صاف قیمت قبول کریں۔']],
  ['03', ['Review the finished job', 'مکمل کام کا ریویو دیں'],
    ['Your worker marks the job complete. Leave a review to help the next neighbour.', 'کاریگر بتاتا ہے کہ کام مکمل ہو گیا۔ ریویو لکھیں تاکہ اگلے پڑوسی کو مدد ملے۔']],
]

export function Landing() {
  const t = useT()
  const ur = useLang() === 'ur'
  return (
    <main id="main-content">
      <section className="home-hero">
        <iframe src="/mesh-gradient.html" title={t('Decorative Mint mesh gradient', 'سجاوٹی پس منظر')} aria-hidden="true" tabIndex={-1} />
        <div className="site-container hero-grid">
          <div className="hero-copy">
            <p className="eyebrow">{t('Islamabad & Rawalpindi · روشن روزگار', 'اسلام آباد اور راولپنڈی · روشن روزگار')}</p>
            <h1>{t('Good hands.', 'اچھے کاریگر۔')}<br /><span>{t('Right around you.', 'آپ کے آس پاس۔')}</span></h1>
            <p className="hero-description">
              {t('A leaking tap. A quiet AC. A job that needs doing.', 'نل ٹپک رہا ہے۔ اے سی بند ہے۔ کوئی کام کروانا ہے۔')}<br />
              {t('Find local skilled workers. Talk it through.', 'قریب کے ماہر کاریگر ڈھونڈیں۔ بات کریں۔')}<br />
              {t('Agree a price, then get it done.', 'قیمت طے کریں، پھر کام کروائیں۔')}
            </p>
            <div className="hero-actions">
              <Link className="button" href="/app/find">{t('Describe your problem', 'اپنا مسئلہ بتائیں')} <Icon name={ur ? 'ArrowLeft' : 'ArrowRight'} /></Link>
              <Link className="worker-entry" href="/worker/login">{t('Here to find work?', 'کام ڈھونڈ رہے ہیں؟')} <span>{t('Join as a worker →', 'کاریگر بن کر شامل ہوں ←')}</span></Link>
            </div>
            <div className="hero-footnote">
              <span><Icon name="Mic" /> {t('Workers answer by voice in Urdu', 'کاریگر اردو میں بول کر جواب دیتے ہیں')}</span>
              <span><Icon name="MapPin" /> {t('Nearest free workers first', 'سب سے قریب فارغ کاریگر پہلے')}</span>
              <span><Icon name="Check" /> {t('Price agreed before booking', 'بکنگ سے پہلے قیمت طے')}</span>
            </div>
          </div>
          <div className="hero-visual">
            <img src="/craft-hero.png" width={1536} height={1024}
              alt={t('Worn hand tools, copper pipes and green leaves arranged on limestone', 'پتھر پر رکھے پرانے اوزار، تانبے کے پائپ اور سبز پتے')} />
            <div className="craft-label">
              <span>{t('Five trades, one request', 'پانچ ہنر، ایک درخواست')}</span>
              <strong>{t('From a leaking tap', 'ٹپکتے نل سے')}<br />{t('to a quiet AC', 'بند اے سی تک')}</strong>
            </div>
          </div>
        </div>
      </section>

      <section className="how-section" id="how-it-works">
        <div className="site-container home-section">
          <div className="section-intro">
            <div><p className="eyebrow">{t('How it works', 'طریقہ کار')}</p><h2>{t('From problem', 'مسئلے سے')}<br />{t('to finished job', 'مکمل کام تک')}</h2></div>
            <p>{t('Stay in control from your first message', 'پہلے میسج سے آخری ریویو تک')}<br />{t('to the final review.', 'سب کچھ آپ کے ہاتھ میں۔')}</p>
          </div>
          <div className="how-grid">
            {steps.map(([n, title, text]) => <article key={n}><span className="step-number" lang="en" dir="ltr">{n}</span><h3>{t(...title)}</h3><p>{t(...text)}</p></article>)}
          </div>
        </div>
      </section>

      <section className="trades-band" id="services">
        <div className="home-section site-container">
          <div className="section-intro">
            <div><p className="eyebrow">{t('Trades', 'ہنر')}</p><h2>{t('Who do you need today?', 'آج آپ کو کس کی ضرورت ہے؟')}</h2></div>
            <p>{t('Five skilled trades.', 'پانچ طرح کے ماہر کاریگر۔')}<br />{t('One easier way to find the right person.', 'صحیح بندہ ڈھونڈنے کا آسان طریقہ۔')}</p>
          </div>
          <div className="trade-grid">
            {TRADE_KEYS.map((k) => (
              <Link key={k} className="service-tile" href="/app/find/confirm"
                onClick={() => problemDraft.set({ trade: k, trade_confidence: 1, description: '', price_range: null })}>
                <span className="service-icon"><Icon name={TRADES[k].icon} /></span>
                <h3>{t(TRADES[k].en, TRADES[k].ur)}</h3>
                <p>{t(TRADES[k].blurb, TRADES[k].blurbUr)}</p>
                <span className="service-arrow"><Icon name="ArrowUpRight" /></span>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="trust-section">
        <div className="site-container">
          <p className="eyebrow">{t('Trust', 'بھروسہ')}</p>
          <h2>{t('Clear prices', 'صاف قیمت')}<br />{t('and reviews from completed jobs', 'اور مکمل کاموں کے ریویو')}</h2>
          <p>{t(
            'Hear a worker’s introduction, look at their experience and read what previous customers have shared. Agree the scope before you book. Workers join by answering four short questions in Urdu.',
            'کاریگر کا تعارف سنیں، اس کا تجربہ دیکھیں اور پڑھیں کہ پہلے گاہکوں نے کیا کہا۔ بکنگ سے پہلے طے کریں کہ کام کیا ہے۔ کاریگر اردو میں چار چھوٹے سوالوں کے جواب دے کر شامل ہوتے ہیں۔',
          )}</p>
          <Link className="button" href="/app/find">{t('Start your first request', 'اپنی پہلی درخواست شروع کریں')}</Link>
        </div>
      </section>
    </main>
  )
}
