import { buildPublishedArtifactBundle } from '../lib/published-artifacts'

async function main() {
  const input = await readJSONFromStdin()
  const bundle = buildPublishedArtifactBundle(input)
  process.stdout.write(JSON.stringify(bundle))
}

async function readJSONFromStdin() {
  const chunks: Uint8Array[] = []

  for await (const chunk of process.stdin) {
    chunks.push(typeof chunk === 'string' ? Buffer.from(chunk) : chunk)
  }

  const raw = Buffer.concat(chunks).toString('utf8').trim()
  if (raw === '') {
    throw new Error('render artifact input is required on stdin')
  }

  return JSON.parse(raw)
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : 'unknown render error'
  process.stderr.write(`${message}\n`)
  process.exitCode = 1
})
