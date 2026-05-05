import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/app/')({
  component: SitesIndex,
})

function SitesIndex() {
  return (
    <div className="builder-panel">
      <div>
        <p className="eyebrow">Builder</p>
        <h1>Sites</h1>
      </div>
      <div className="empty-state">
        <p>No sites yet.</p>
        <button type="button" disabled>
          New site
        </button>
      </div>
    </div>
  )
}
