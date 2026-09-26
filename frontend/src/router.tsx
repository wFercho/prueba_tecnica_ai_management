import { createRootRoute, createRoute, createRouter, Link, redirect } from '@tanstack/react-router'
import App from './App'
import { api, ApiError } from './api'
import { Dashboard } from './views/Dashboard'
import { MeterView } from './views/MeterView'
import { AnomalyView } from './views/AnomalyView'
import { LoginView } from './views/LoginView'

const rootRoute = createRootRoute({
  component: App,
  notFoundComponent: () => (
    <div className="card">
      <h1>Página no encontrada</h1>
      <Link to="/">Volver al panel general</Link>
    </div>
  ),
})

const requireSession = async ({ location }: { location: { href: string } }) => {
  try {
    await api.session()
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      throw redirect({ to: '/login', search: { redirect: location.href } })
    }
    throw error
  }
}

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>) => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : '/',
  }),
  component: LoginView,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Dashboard,
  beforeLoad: requireSession,
})

const meterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/medidores/$meterId',
  component: () => <MeterView meterId={meterRoute.useParams().meterId} />,
  beforeLoad: requireSession,
})

const anomalyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/hallazgos/$anomalyId',
  component: () => <AnomalyView anomalyId={Number(anomalyRoute.useParams().anomalyId)} />,
  beforeLoad: requireSession,
})

const routeTree = rootRoute.addChildren([dashboardRoute, meterRoute, anomalyRoute, loginRoute])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
