import { Icon } from '../components/Icon'
import { problemDraft } from '../lib/draft'
import { Link } from '../lib/router'
import { TRADE_KEYS, TRADES } from '../lib/trades'

const steps = [
  ['01', 'Tell us what’s wrong', 'Type or speak in English, Urdu or Roman Urdu. We help you find the right trade.'],
  ['02', 'Agree a price in chat', 'Choose an available worker nearby. Discuss the job and accept a clear price offer.'],
  ['03', 'Review the finished job', 'Your worker marks the job complete. Leave a review to help the next neighbour.'],
]

export function Landing() {
  return (
    <main id="main-content">
      <section className="home-hero">
        <iframe src="/mesh-gradient.html" title="Decorative Mint mesh gradient" aria-hidden="true" tabIndex={-1} />
        <div className="site-container hero-grid">
          <div className="hero-copy">
            <p className="eyebrow">Islamabad &amp; Rawalpindi · روشن روزگار</p>
            <h1>Good hands.<br /><span>Right around you.</span></h1>
            <p className="hero-description">A leaking tap. A quiet AC. A job that needs doing.<br />Find local skilled workers. Talk it through.<br />Agree a price, then get it done.</p>
            <div className="hero-actions">
              <Link className="button" href="/app/find">Describe your problem <Icon name="ArrowRight" /></Link>
              <Link className="worker-entry" href="/worker/login">Here to find work? <span>Join as a worker →</span></Link>
            </div>
            <div className="hero-footnote">
              <span><Icon name="Mic" /> Workers answer by voice in Urdu</span>
              <span><Icon name="MapPin" /> Nearest free workers first</span>
              <span><Icon name="Check" /> Price agreed before booking</span>
            </div>
          </div>
          <div className="hero-visual">
            <img src="/craft-hero.png" width={1536} height={1024} alt="Worn hand tools, copper pipes and green leaves arranged on limestone" />
            <div className="craft-label"><span>Five trades, one request</span><strong>From a leaking tap<br />to a quiet AC</strong></div>
          </div>
        </div>
      </section>

      <section className="how-section" id="how-it-works">
        <div className="site-container home-section">
          <div className="section-intro">
            <div><p className="eyebrow">How it works</p><h2>From problem<br />to finished job</h2></div>
            <p>Stay in control from your first message<br />to the final review.</p>
          </div>
          <div className="how-grid">
            {steps.map(([n, t, d]) => <article key={n}><span className="step-number">{n}</span><h3>{t}</h3><p>{d}</p></article>)}
          </div>
        </div>
      </section>

      <section className="trades-band" id="services">
        <div className="home-section site-container">
          <div className="section-intro">
            <div><p className="eyebrow">Trades</p><h2>Who do you need today?</h2></div>
            <p>Five skilled trades.<br />One easier way to find the right person.</p>
          </div>
          <div className="trade-grid">
            {TRADE_KEYS.map((t) => (
              <Link key={t} className="service-tile" href="/app/find/confirm"
                onClick={() => problemDraft.set({ trade: t, trade_confidence: 1, description: '', price_range: null })}>
                <span className="service-icon"><Icon name={TRADES[t].icon} /></span>
                <h3>{TRADES[t].en}</h3>
                <p>{TRADES[t].blurb}</p>
                <span className="service-arrow"><Icon name="ArrowUpRight" /></span>
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="trust-section">
        <div className="site-container">
          <p className="eyebrow">Trust</p>
          <h2>Clear prices<br />and reviews from completed jobs</h2>
          <p>Hear a worker’s introduction, look at their experience and read what previous customers have shared. Agree the scope before you book. Workers join by answering four short questions in Urdu.</p>
          <Link className="button" href="/app/find">Start your first request</Link>
        </div>
      </section>
    </main>
  )
}
