import { Link, Outlet, useLocation } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from './api'
import './App.css'

export default function App() {
  const location = useLocation()
  const queryClient = useQueryClient()
  const session = useQuery({ queryKey: ['session'], queryFn: api.session, enabled: location.pathname !== '/login' })
  const logout = useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      queryClient.clear()
      window.location.replace('/login')
    },
  })
  return (
    <div className="app mx-auto max-w-6xl px-4 pb-16 pt-7 sm:px-6">
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-5">
        <div>
          <Link to="/" className="text-xl font-semibold tracking-tight text-slate-900 no-underline">
            Gestión energética
          </Link>
          <p className="mt-1 text-sm text-slate-600">
            Lecturas horarias y análisis de medidores
          </p>
        </div>
        {location.pathname !== '/login' && (
          <nav aria-label="Navegación principal" className="flex flex-wrap items-center gap-3">
            <Link to="/" className="rounded-md px-3 py-2 text-sm font-medium text-blue-700 hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">
              Panel general
            </Link>
            {session.data && <span className="text-sm text-slate-600">{session.data.user.email}</span>}
            <button className="action" type="button" disabled={logout.isPending} onClick={() => logout.mutate()}>Cerrar sesión</button>
          </nav>
        )}
      </header>
      {logout.isError && <p className="notice error" role="alert">No se pudo cerrar la sesión. Vuelve a intentarlo.</p>}
      <main><Outlet /></main>
    </div>
  )
}
