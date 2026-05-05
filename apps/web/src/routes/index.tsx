import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/')({
  component: Home,
})

function Home() {
  return (
    <main className="page-shell home-grid">
      <section className="intro-panel">
        <p className="eyebrow">Prototype</p>
        <h1>Structured site drafts from prompts.</h1>
        <p>
          Snaelda turns a prompt into validated site data, then keeps editing,
          preview, and publish on the same draft contract.
        </p>
      </section>

      <form className="prompt-panel">
        <label htmlFor="site-prompt">Website prompt</label>
        <textarea
          id="site-prompt"
          name="prompt"
          rows={8}
          placeholder="A five-page site for a local design studio"
        />
        <button type="button" disabled>
          Create draft
        </button>
      </form>
    </main>
  )
}
