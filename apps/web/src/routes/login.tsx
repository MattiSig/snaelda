import { createFileRoute } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { APIError, login } from '@/lib/api'
import { cn } from '@/lib/utils'
import { form, layout, paddedPanel, text } from '@/lib/styles'

export const Route = createFileRoute('/login')({
  validateSearch: (search: Record<string, unknown>) => ({
    redirect:
      typeof search.redirect === 'string' &&
      search.redirect.startsWith('/') &&
      !search.redirect.startsWith('//')
        ? search.redirect
        : '/app',
  }),
  component: Login,
})

function Login() {
  const search = Route.useSearch()
  const [email, setEmail] = useState('demo@snaelda.local')
  const [name, setName] = useState('Demo User')
  const [errorMessage, setErrorMessage] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage('')
    setIsSubmitting(true)

    try {
      await login(email, name)
      window.location.href = search.redirect
    } catch (error) {
      if (error instanceof APIError) {
        setErrorMessage(error.message)
      } else {
        setErrorMessage('Could not sign in')
      }
      setIsSubmitting(false)
    }
  }

  return (
    <main className={cn(layout.pageShell, layout.narrowShell, 'pt-14')}>
      <form className={cn(paddedPanel, form.grid, 'relative')} onSubmit={handleSubmit}>
        <img
          src="/logo.png"
          alt=""
          className="mb-2 size-16 rounded-[18px] border border-border bg-[var(--surface-2)] object-contain p-2"
        />
        <div className="mb-3">
          <p className={text.eyebrow}>Builder access</p>
          <h1 className={cn(text.h1, 'max-w-[9ch]')}>Sign in</h1>
        </div>
        <label htmlFor="email" className={text.label}>Email</label>
        <Input
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />
        <label htmlFor="name" className={text.label}>Name</label>
        <Input
          id="name"
          name="name"
          type="text"
          autoComplete="name"
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Signing in...' : 'Sign in'}
        </Button>
      </form>
    </main>
  )
}
