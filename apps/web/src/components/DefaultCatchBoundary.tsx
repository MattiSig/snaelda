import type { ErrorComponentProps } from '@tanstack/react-router'

export function DefaultCatchBoundary({ error }: ErrorComponentProps) {
  return (
    <main className="page-shell">
      <section className="status-panel">
        <p className="eyebrow">Error</p>
        <h1>Something went wrong.</h1>
        <p>{error.message}</p>
      </section>
    </main>
  )
}
