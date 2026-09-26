import { useEffect, useRef, useState } from 'react'
import { ChevronDown, LogOut, User } from 'lucide-react'

// UserMenu groups the session identity and the logout action behind one
// control. The trigger shows the account's local name; the menu reveals the
// full email and the logout action. Escape closes returning focus to the
// trigger, and a pointer press outside closes as well.
export function UserMenu({
  email,
  loggingOut,
  onLogout,
}: {
  email: string
  loggingOut: boolean
  onLogout: () => void
}) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const localPart = email.split('@')[0] || email

  useEffect(() => {
    if (!open) return
    const onPointer = (event: PointerEvent) => {
      if (root.current && !root.current.contains(event.target as Node)) setOpen(false)
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false)
        trigger.current?.focus()
      }
    }
    document.addEventListener('pointerdown', onPointer)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('pointerdown', onPointer)
      document.removeEventListener('keydown', onKey)
    }
  }, [open ])

  return (
    <div ref={root} className="relative">
      <button
        ref={trigger}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className="inline-flex max-w-48 items-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
      >
        <User size={16} aria-hidden="true" className="shrink-0" />
        <span className="truncate">{localPart}</span>
        <ChevronDown size={14} aria-hidden="true" className="shrink-0 text-slate-400" />
      </button>
      {open && (
        <div role="menu" aria-label="Cuenta" className="absolute right-0 z-10 mt-1 w-64 rounded-lg bg-white py-1 shadow-md">
          <p className="truncate px-4 py-2 text-sm text-slate-500">{email}</p>
          <button
            role="menuitem"
            type="button"
            disabled={loggingOut}
            onClick={onLogout}
            className="inline-flex w-full items-center gap-2 px-4 py-2 text-left text-sm font-medium text-slate-700 hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700 disabled:opacity-50"
          >
            <LogOut size={16} aria-hidden="true" />
            {loggingOut ? 'Cerrando sesión…' : 'Cerrar sesión'}
          </button>
        </div>
      )}
    </div>
  )
}
