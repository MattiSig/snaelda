import { Link, createFileRoute } from '@tanstack/react-router'
import type { FormEvent } from 'react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { APIError, login } from '@/lib/api'
import { cn } from '@/lib/utils'
import { form, layout, paddedPanel, text } from '@/lib/styles'
import { translator } from '@/lib/i18n'
import { useLocale } from '@/lib/locale'

export const Route = createFileRoute('/login')({
  component: Login,
})

function Login() {
  const tr = translator(useLocale())
  const [email, setEmail] = useState('')
  const [message, setMessage] = useState('')
  const [errorMessage, setErrorMessage] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setErrorMessage('')
    setMessage('')
    setIsSubmitting(true)

    try {
      const response = await login(email)
      setMessage(response.message)
    } catch (error) {
      setErrorMessage(
        error instanceof APIError ? error.message : tr('login.error.magicLink'),
      )
    } finally {
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
          <p className={text.eyebrow}>{tr('login.eyebrow')}</p>
          <h1 className={cn(text.h1, 'max-w-[11ch]')}>{tr('login.title')}</h1>
        </div>
        <label htmlFor="email" className={text.label}>{tr('login.emailLabel')}</label>
        <Input
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          required
        />
        <p className="text-sm text-[var(--paper-muted)]">
          {tr('login.helper')}
        </p>
        {errorMessage ? <p className={text.error}>{errorMessage}</p> : null}
        {message ? <p className={text.success}>{message}</p> : null}
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? tr('login.submitSending') : tr('login.submit')}
        </Button>
        <p className="text-xs text-[var(--paper-muted)]">
          {tr('login.consent.prefix')}{' '}
          <Link
            to="/terms"
            className="font-semibold underline underline-offset-4 hover:text-[var(--paper)]"
          >
            {tr('login.consent.terms')}
          </Link>{' '}
          {tr('login.consent.and')}{' '}
          <Link
            to="/privacy"
            className="font-semibold underline underline-offset-4 hover:text-[var(--paper)]"
          >
            {tr('login.consent.privacy')}
          </Link>
          .
        </p>
      </form>
    </main>
  )
}
