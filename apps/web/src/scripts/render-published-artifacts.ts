import readline from 'node:readline'
import { buildPublishedArtifactBundle } from '../lib/published-artifacts'

const ONESHOT_FLAG = '--oneshot'

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

async function runOneshot() {
  const input = await readJSONFromStdin()
  const bundle = buildPublishedArtifactBundle(input)
  process.stdout.write(JSON.stringify(bundle))
}

function runWorker() {
  // Worker protocol: newline-delimited JSON over stdin/stdout.
  // Each input line is a JSON object: { "id": string, "input": <PublishedArtifactRenderInput> }.
  // Each response is a JSON object on its own line: { "id": string, "bundle"?: ..., "error"?: string }.
  // JSON.stringify on the response side guarantees no embedded raw newlines, so newline framing is safe.
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: false,
    crlfDelay: Infinity,
  })

  process.stdin.on('end', () => {
    process.exit(0)
  })

  rl.on('line', (line) => {
    const trimmed = line.trim()
    if (trimmed === '') {
      return
    }

    let request: { id?: unknown; input?: unknown }
    try {
      request = JSON.parse(trimmed)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'invalid render request payload'
      process.stdout.write(JSON.stringify({ error: `decode render request: ${message}` }) + '\n')
      return
    }

    const id = typeof request.id === 'string' ? request.id : ''
    if (!request.input) {
      process.stdout.write(JSON.stringify({ id, error: 'render request must include an input payload' }) + '\n')
      return
    }

    try {
      const bundle = buildPublishedArtifactBundle(request.input as Parameters<typeof buildPublishedArtifactBundle>[0])
      process.stdout.write(JSON.stringify({ id, bundle }) + '\n')
    } catch (error) {
      const message = error instanceof Error ? error.message : 'unknown render error'
      process.stdout.write(JSON.stringify({ id, error: message }) + '\n')
    }
  })
}

async function main() {
  if (process.argv.includes(ONESHOT_FLAG)) {
    await runOneshot()
    return
  }
  runWorker()
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : 'unknown render error'
  process.stderr.write(`${message}\n`)
  process.exitCode = 1
})
