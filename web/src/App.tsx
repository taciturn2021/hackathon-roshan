import { Fragment, useEffect, type ReactNode } from 'react'
import { AppShell } from './components/AppShell'
import { Shell } from './components/Shell'
import { Task } from './components/Task'
import { StatePanel } from './components/ui'
import { Link, match, Redirect, useLocation } from './lib/router'
import { useMe } from './lib/session'
import { CustomerSignIn, RolePick, WorkerSignIn } from './screens/Auth'
import { Dashboard } from './screens/Dashboard'
import { CustomerLocation, Problem, ProblemConfirm, WorkerList } from './screens/Find'
import { CancelJob, Chat, JobStatus, MakeOffer, ReviewJob } from './screens/Job'
import { Jobs } from './screens/Jobs'
import { Landing } from './screens/Landing'
import { Onboard, WorkerConfirm, WorkerLocation } from './screens/Onboarding'
import { OwnProfile, WorkerProfile } from './screens/Profile'

type Params = Record<string, string>
type Route = { path: string; title: string; ur?: string; page: (p: Params) => ReactNode }

const PUBLIC: Route[] = [
  { path: '/', title: 'Good hands. Right around you.', page: () => <Landing /> },
  { path: '/start', title: 'Get started', page: () => <RolePick /> },
  { path: '/login', title: 'Sign in', page: () => <CustomerSignIn mode="signin" /> },
  { path: '/signup', title: 'Create account', page: () => <CustomerSignIn mode="signup" /> },
  { path: '/worker/login', title: 'Worker sign in', page: () => <WorkerSignIn /> },
  { path: '/worker/onboard', title: 'Worker onboarding', page: () => <Onboard /> },
  { path: '/worker/location', title: 'Your location', page: () => <WorkerLocation /> },
  { path: '/worker/confirm', title: 'Confirm your profile', page: () => <WorkerConfirm /> },
]

const APP: Route[] = [
  { path: '/app', title: 'Dashboard', ur: 'کام', page: () => <Dashboard /> },
  { path: '/app/find', title: 'Find a worker', page: () => <Problem /> },
  { path: '/app/find/confirm', title: 'Find a worker', page: () => <ProblemConfirm /> },
  { path: '/app/find/location', title: 'Find a worker', page: () => <CustomerLocation /> },
  { path: '/app/workers', title: 'Workers near you', page: () => <WorkerList /> },
  { path: '/app/workers/:id', title: 'Worker profile', page: (p) => <WorkerProfile id={Number(p.id)} /> },
  { path: '/app/jobs', title: 'Jobs', ur: 'میرے کام', page: () => <Jobs /> },
  { path: '/app/jobs/:id', title: 'Job chat', ur: 'بات چیت', page: (p) => <Chat id={Number(p.id)} /> },
  { path: '/app/jobs/:id/offer', title: 'Offer a price', ur: 'قیمت', page: (p) => <MakeOffer id={Number(p.id)} /> },
  { path: '/app/jobs/:id/status', title: 'Job', ur: 'کام', page: (p) => <JobStatus id={Number(p.id)} /> },
  { path: '/app/jobs/:id/cancel', title: 'Cancel job', ur: 'منسوخ', page: (p) => <CancelJob id={Number(p.id)} /> },
  { path: '/app/jobs/:id/review', title: 'Leave a review', page: (p) => <ReviewJob id={Number(p.id)} /> },
  { path: '/app/profile', title: 'Profile', ur: 'پروفائل', page: () => <OwnProfile /> },
]

/** Old addresses, kept so saved links still work. */
const MOVED: Record<string, string> = { '/signin': '/login', '/worker/signin': '/worker/login', '/worker': '/app', '/find': '/app/find', '/jobs': '/app/jobs' }
/** Signed-in visitors skip these and go straight to the app. */
const SIGNED_OUT_ONLY = ['/', '/login', '/signup', '/worker/login', '/start']

function find(routes: Route[], path: string) {
  for (const r of routes) {
    const params = match(r.path, path)
    if (params && Object.values(params).every((v) => /^\d+$/.test(v))) return { route: r, params }
  }
  return null
}

export default function App() {
  const { path, query } = useLocation()
  const me = useMe()
  const inApp = path === '/app' || path.startsWith('/app/')
  const hit = find(inApp ? APP : PUBLIC, path)
  const worker = me?.user.role === 'worker'
  const title = hit ? (worker && hit.route.ur) || hit.route.title : 'Page not found'
  useEffect(() => { document.title = `${title} · Roshan Rozgar` }, [title])

  if (MOVED[path]) return <Redirect to={MOVED[path] + (query.size ? `?${query}` : '')} />
  if (inApp && !me) return <Redirect to={`/login?next=${encodeURIComponent(path + (query.size ? `?${query}` : ''))}`} />
  if (inApp && worker && !me?.worker) return <Redirect to="/worker/onboard" />
  if (!inApp && me && SIGNED_OUT_ONLY.includes(path)) return <Redirect to={query.get('next') ?? '/app'} />

  if (hit && inApp) return <AppShell title={title}><Fragment key={path}>{hit.route.page(hit.params)}</Fragment></AppShell>
  if (hit) return <Shell key={path}>{hit.route.page(hit.params)}</Shell>
  return (
    <Shell>
      <Task screen="not-found" wide>
        <StatePanel kind="error" icon="CircleHelp" title="Page not found" text="This page doesn’t exist." />
        <Link className="button full" href="/">Go home</Link>
      </Task>
    </Shell>
  )
}
