import { layout, paddedPanel, text } from '@/lib/styles'

export function NotFound() {
  return (
    <main className={layout.pageShell}>
      <section className={paddedPanel}>
        <p className={text.eyebrow}>404</p>
        <h1 className={text.h1}>Page not found.</h1>
      </section>
    </main>
  )
}
