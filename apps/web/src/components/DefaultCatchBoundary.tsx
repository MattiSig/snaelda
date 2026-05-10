import type { ErrorComponentProps } from '@tanstack/react-router'
import { layout, paddedPanel, text } from '@/lib/styles'

export function DefaultCatchBoundary({ error }: ErrorComponentProps) {
  return (
    <main className={layout.pageShell}>
      <section className={paddedPanel}>
        <p className={text.eyebrow}>Error</p>
        <h1 className={text.h1}>Something went wrong.</h1>
        <p className={text.p}>{error.message}</p>
      </section>
    </main>
  )
}
