import { createFileRoute } from '@tanstack/react-router'
import { layout, paddedPanel, preview, text } from '@/lib/styles'

export const Route = createFileRoute('/preview/$token')({
  component: TokenPreview,
})

function TokenPreview() {
  const { token } = Route.useParams()

  return (
    <main className={layout.pageShell}>
      <section className={paddedPanel}>
        <div className={preview.toolbar}>
          <p className={text.eyebrow}>Preview token</p>
          <strong>{token}</strong>
        </div>
        <article className="mt-5 rounded-[16px] border border-border bg-[var(--surface-2)] p-6">
          <h1 className={text.h1}>Preview route</h1>
        </article>
      </section>
    </main>
  )
}
