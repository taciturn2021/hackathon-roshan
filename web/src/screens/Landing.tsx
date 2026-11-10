import { Icon, type IconName } from '../components/Icon'
import { problemDraft } from '../lib/draft'
import { useLang, useT } from '../lib/i18n'
import { Link } from '../lib/router'
import { TRADE_KEYS, TRADES } from '../lib/trades'

type Pair = [string, string] // [English, Urdu]
type Step = { icon: IconName; title: Pair; text: Pair }

const CUSTOMER_STEPS: Step[] = [
  { icon: 'Mic', title: ['Say what is wrong', 'بتائیں کیا خراب ہے'], text: ['Type it or speak it, in English, Urdu or Roman Urdu.', 'لکھیں یا بولیں، اردو، رومن اردو یا انگریزی میں۔'] },
  { icon: 'MapPin', title: ['See who is free nearby', 'دیکھیں قریب کون فارغ ہے'], text: ['Only workers who can come now, nearest first.', 'صرف وہ کاریگر جو ابھی آ سکتے ہیں، سب سے قریب والا پہلے۔'] },
  { icon: 'Banknote', title: ['Agree the price, then book', 'قیمت طے کریں، پھر بک کریں'], text: ['Nothing is booked until one of you accepts the price.', 'جب تک قیمت منظور نہ ہو، کچھ بک نہیں ہوتا۔'] },
]
const WORKER_STEPS: Step[] = [
  { icon: 'Mic', title: ['Answer 4 questions by voice', 'بول کر 4 سوالوں کے جواب دیں'], text: ['No typing and no forms. Your profile is made from your answers.', 'نہ لکھنا، نہ فارم بھرنا۔ آپ کے جوابوں سے پروفائل بن جاتا ہے۔'] },
  { icon: 'Bell', title: ['Go online, jobs come to you', 'آن لائن ہوں، کام خود آئے گا'], text: ['A bell rings and the job is read out to you in Urdu.', 'گھنٹی بجتی ہے اور کام اردو میں سنایا جاتا ہے۔'] },
  { icon: 'CircleCheck', title: ['Send your price, do the work', 'قیمت بھیجیں، کام کریں'], text: ['One job at a time, so nobody is double booked.', 'ایک وقت میں ایک کام، تاکہ کوئی دو جگہ بک نہ ہو۔'] },
]
const FACTS: { icon: IconName; text: Pair }[] = [
  { icon: 'Clock', text: ['Only workers who are free right now are shown', 'صرف وہی کاریگر دکھائے جاتے ہیں جو ابھی فارغ ہیں'] },
  { icon: 'Banknote', text: ['The price is agreed in chat before anything is booked', 'بکنگ سے پہلے چیٹ میں قیمت طے ہوتی ہے'] },
  { icon: 'Star', text: ['Reviews come only from completed jobs', 'ریویو صرف مکمل کاموں کے بعد دیے جاتے ہیں'] },
  { icon: 'Volume2', text: ['Workers can hear every job and message in Urdu', 'کاریگر ہر کام اور میسج اردو میں سن سکتے ہیں'] },
]

export function Landing() {
  const t = useT()
  const ur = useLang() === 'ur'
  const forward: IconName = ur ? 'ArrowLeft' : 'ArrowRight'
  const steps = (list: Step[]) => (
    <ol className="lp-steps">
      {list.map((s, i) => (
        <li key={s.title[0]}>
          <span className="lp-step-icon"><Icon name={s.icon} /></span>
          <div><h3><span className="digits">{i + 1}.</span> {t(...s.title)}</h3><p>{t(...s.text)}</p></div>
        </li>
      ))}
    </ol>
  )

  return (
    <main id="main-content" className="lp">
      <section className="lp-hero">
        <div className="site-container lp-hero-grid">
          <div className="lp-hero-copy">
            <p className="lp-kicker">{t('Islamabad & Rawalpindi', 'اسلام آباد اور راولپنڈی')}</p>
            <h1>
              {t('Tap leaking? AC not cooling?', 'نل ٹپک رہا ہے؟ اے سی ٹھنڈا نہیں کر رہا؟')}
              <span>{t('A good worker is a few streets away.', 'اچھا کاریگر چند گلیاں دور ہے۔')}</span>
            </h1>
            <p className="lp-lede">{t(
              'Tell us the problem, see who is free near you right now, and agree the price before you book.',
              'مسئلہ بتائیں، دیکھیں کہ آپ کے قریب ابھی کون فارغ ہے، اور بک کرنے سے پہلے قیمت طے کریں۔',
            )}</p>
            <div className="lp-doors">
              <Link className="lp-door primary" href="/app/find">
                <span className="lp-door-icon"><Icon name="House" /></span>
                <span><strong>{t('I need a worker', 'مجھے کاریگر چاہیے')}</strong><small>{t('Describe your problem', 'اپنا مسئلہ بتائیں')}</small></span>
                <Icon name={forward} />
              </Link>
              <Link className="lp-door" href="/worker/login">
                <span className="lp-door-icon"><Icon name="Wrench" /></span>
                <span><strong>{t('I am a worker', 'میں کاریگر ہوں')}</strong><small>{t('Join by voice, get jobs', 'بول کر شامل ہوں، کام لیں')}</small></span>
                <Icon name={forward} />
              </Link>
            </div>
          </div>

          {/* The product itself, not a stock picture: one request from first message to agreed price. */}
          <figure className="lp-demo" aria-label={t('Example of one request', 'ایک درخواست کی مثال')}>
            <figcaption>{t('Example request', 'مثال')}</figcaption>
            <p className="lp-bubble" dir="auto">Kitchen ka nalka leak ho raha hai, abhi koi aa jaye</p>
            <div className="lp-chip-row">
              <span className="lp-chip"><Icon name={TRADES.plumber.icon} />{t(TRADES.plumber.en, 'پلمبر')}</span>
              <span className="lp-chip warn">{t('Now', 'ابھی')}</span>
              <span className="lp-chip digits">Rs 1,250–2,300</span>
            </div>
            <div className="lp-worker">
              <span className="avatar">AR</span>
              <div>
                <strong lang="en" dir="ltr">Abdul Rehman</strong>
                <small><span className="digits">1.2 km</span> · <Icon name="Star" className="icon-sm" /> <span className="digits">4.7</span> · {t('free now', 'ابھی فارغ')}</small>
              </div>
            </div>
            <div className="lp-offer">
              <span>{t('His price', 'ان کی قیمت')}</span>
              <strong className="digits">Rs 1,500</strong>
              <span className="lp-accept"><Icon name="Check" />{t('Accept', 'منظور')}</span>
            </div>
          </figure>
        </div>
      </section>

      <section className="lp-section" id="how-it-works">
        <div className="site-container">
          <h2>{t('How it works', 'یہ کیسے کام کرتا ہے')}</h2>
          <div className="lp-two">
            <div className="lp-panel">
              <p className="lp-panel-tag"><Icon name="House" />{t('If you need help at home', 'اگر آپ کو گھر پر کام کروانا ہے')}</p>
              {steps(CUSTOMER_STEPS)}
              <Link className="button full" href="/app/find">{t('Describe your problem', 'اپنا مسئلہ بتائیں')}<Icon name={forward} /></Link>
            </div>
            <div className="lp-panel worker-side">
              <p className="lp-panel-tag"><Icon name="Wrench" />{t('If you are a skilled worker', 'اگر آپ کاریگر ہیں')}</p>
              {steps(WORKER_STEPS)}
              <Link className="button outline full" href="/worker/login">{t('Join as a worker', 'کاریگر بن کر شامل ہوں')}<Icon name={forward} /></Link>
            </div>
          </div>
        </div>
      </section>

      <section className="lp-section tinted" id="services">
        <div className="site-container">
          <h2>{t('Who do you need today?', 'آج آپ کو کس کی ضرورت ہے؟')}</h2>
          <div className="trade-grid">
            {TRADE_KEYS.map((k) => (
              <Link key={k} className="service-tile" href="/app/find/confirm"
                onClick={() => problemDraft.set({ trade: k, trade_confidence: 1, description: '', price_range: null })}>
                <span className="service-icon"><Icon name={TRADES[k].icon} /></span>
                <h3>{t(TRADES[k].en, TRADES[k].ur)}</h3>
                <p>{t(TRADES[k].blurb, TRADES[k].blurbUr)}</p>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="lp-section">
        <div className="site-container">
          <h2>{t('Fair for both sides', 'دونوں کے لیے منصفانہ')}</h2>
          <ul className="lp-facts">
            {FACTS.map((f) => <li key={f.text[0]}><Icon name={f.icon} /><span>{t(...f.text)}</span></li>)}
          </ul>
        </div>
      </section>
    </main>
  )
}
