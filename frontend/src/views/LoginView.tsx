import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useSearch } from '@tanstack/react-router'
import { api } from '../api'

export function LoginView() {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const queryClient = useQueryClient()
  const { redirect } = useSearch({ from: '/login' })
  const login = useMutation({
    mutationFn: () => api.login(email, password),
    onSuccess: (session) => {
      queryClient.setQueryData(['session'], session)
      const destination = redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/'
      window.location.assign(destination)
    },
  })
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    login.mutate()
  }

  return (
    <section className="card mx-auto mt-12 max-w-md" aria-labelledby="login-title">
      <h1 id="login-title">Iniciar sesión</h1>
      <p className="muted">Accede a la demostración local de gestión energética.</p>
      <form className="flex flex-col gap-4" onSubmit={submit}>
        <div className="flex flex-col gap-1">
          <label htmlFor="email">Correo electrónico</label>
          <input id="email" type="email" autoComplete="username" required value={email}
            onChange={(event) => setEmail(event.target.value)} className="rounded-md border border-slate-300 p-3" />
        </div>
        <div className="flex flex-col gap-1">
          <label htmlFor="password">Contraseña</label>
          <input id="password" type="password" autoComplete="current-password" required value={password}
            onChange={(event) => setPassword(event.target.value)} className="rounded-md border border-slate-300 p-3" />
        </div>
        {login.isError && <p role="alert" className="notice error">{login.error.message} Comprueba los datos e inténtalo de nuevo.</p>}
        <button type="submit" className="action primary" disabled={login.isPending}>
          {login.isPending ? 'Entrando…' : 'Entrar'}
        </button>
      </form>
    </section>
  )
}
