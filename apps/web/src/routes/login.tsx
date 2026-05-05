import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/login')({
  component: Login,
})

function Login() {
  return (
    <main className="page-shell narrow-shell">
      <form className="auth-panel">
        <h1>Sign in</h1>
        <label htmlFor="email">Email</label>
        <input id="email" name="email" type="email" autoComplete="email" />
        <label htmlFor="password">Password</label>
        <input
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
        />
        <button type="button" disabled>
          Sign in
        </button>
      </form>
    </main>
  )
}
