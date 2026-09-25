import { createRootRoute, createRoute, createRouter, Link } from '@tanstack/react-router'
import App from './App'
import { Dashboard } from './views/Dashboard'
import { MeterView } from './views/MeterView'
import { AnomalyView } from './views/AnomalyView'

const rootRoute = createRootRoute({
  component: App,
  notFoundComponent: () => (
    <div className="card">
      <h1>Página no encontrada</h1>
      <Link to="/">Volver al panel general</Link>
    </div>
  ),
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Dashboard,
})

const meterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/medidores/$meterId',
  component: () => <MeterView meterId={meterRoute.useParams().meterId} />,
})

const anomalyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/hallazgos/$anomalyId',
  component: () => <AnomalyView anomalyId={Number(anomalyRoute.useParams().anomalyId)} />,
})

const routeTree = rootRoute.addChildren([dashboardRoute, meterRoute, anomalyRoute])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
