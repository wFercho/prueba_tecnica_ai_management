import { Link, Outlet } from '@tanstack/react-router'
import './App.css'

export default function App() {
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
        <nav aria-label="Navegación principal">
          <Link to="/" className="rounded-md px-3 py-2 text-sm font-medium text-blue-700 hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">
            Panel general
          </Link>
        </nav>
      </header>
      <main><Outlet /></main>
    </div>
  )
}
