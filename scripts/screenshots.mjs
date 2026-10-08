#!/usr/bin/env node
// Regenerates the README screenshots from a running demo (make demo).
//
// It signs in through the API, then drives headless Chrome over the DevTools
// protocol to set the session cookie and the color scheme for each page.
// No dependency: Node 22+ (fetch, WebSocket) and Google Chrome.
//
//   CHROME=/path/to/chrome BASE=http://localhost:8080 node scripts/screenshots.mjs
import { spawn } from 'node:child_process'
import { mkdtempSync, readFileSync, writeFileSync, existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const CHROME = process.env.CHROME ?? '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
const BASE = process.env.BASE ?? 'http://localhost:8080'
const EMAIL = process.env.OBSRV_EMAIL ?? 'admin@obsrv.local'
const PASSWORD = process.env.OBSRV_PASSWORD ?? 'obsrv-demo-password'
const OUT = 'docs/images'
const RANGE = 'from=now-30m&to=now'
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// 1. Sign in and keep the session token.
const login = await fetch(`${BASE}/api/v1/auth/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: EMAIL, password: PASSWORD }),
})
if (!login.ok) throw new Error(`sign in failed: ${login.status} ${await login.text()}`)
const session = /obsrv_session=([^;]+)/.exec(login.headers.get('set-cookie') ?? '')?.[1]
if (!session) throw new Error('no session cookie in the sign-in response')
const auth = { headers: { Cookie: `obsrv_session=${session}` } }

// 2. Pick a slow, failing checkout trace for the trace screenshot.
const traces = await (await fetch(`${BASE}/api/v1/traces?${RANGE}&errors=true&min_duration_ms=500&limit=20`, auth)).json()
const trace = traces.data?.find((t) => t.span_count >= 9) ?? traces.data?.[0]
if (!trace) throw new Error('no slow failing trace yet: let the demo run a few minutes')

// And the card declines of the payment service for the Issues screenshot.
const issues = await (await fetch(`${BASE}/api/v1/issues?${RANGE}`, auth)).json()
const issue = issues.data?.find((i) => i.service === 'payment' && i.kind === 'span') ?? issues.data?.[0]
if (!issue) throw new Error('no issue yet: let the demo run a few minutes')

// 3. Start Chrome with remote debugging on a free port.
const profile = mkdtempSync(join(tmpdir(), 'obsrv-shots-'))
const chrome = spawn(CHROME, ['--headless=new', '--disable-gpu', '--hide-scrollbars', '--remote-debugging-port=0',
  `--user-data-dir=${profile}`, 'about:blank'], { stdio: 'ignore' })
let port
for (let i = 0; i < 100 && !port; i++) {
  const file = join(profile, 'DevToolsActivePort')
  if (existsSync(file)) port = readFileSync(file, 'utf8').split('\n')[0]
  else await sleep(100)
}
if (!port) throw new Error('Chrome did not start')
const { webSocketDebuggerUrl } = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json()

// Minimal CDP client.
const ws = new WebSocket(webSocketDebuggerUrl)
await new Promise((resolve, reject) => {
  ws.onopen = resolve
  ws.onerror = reject
})
let nextId = 0
const pending = new Map()
const listeners = []
ws.onmessage = (e) => {
  const msg = JSON.parse(e.data)
  if (msg.id && pending.has(msg.id)) {
    const { resolve, reject } = pending.get(msg.id)
    pending.delete(msg.id)
    msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result)
  } else if (msg.method) listeners.forEach((l) => l(msg))
}
const send = (method, params = {}, sessionId) =>
  new Promise((resolve, reject) => {
    const id = ++nextId
    pending.set(id, { resolve, reject })
    ws.send(JSON.stringify({ id, method, params, sessionId }))
  })

const { targetId } = await send('Target.createTarget', { url: 'about:blank' })
const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true })
const page = (method, params) => send(method, params, sessionId)
await page('Page.enable')
await page('Network.enable')
await page('Network.setCookie', { name: 'obsrv_session', value: session, url: BASE, httpOnly: true })

// act runs in the page before the capture, e.g. to open a menu.
async function shot(name, path, height, { scheme = 'dark', act } = {}) {
  await page('Emulation.setDeviceMetricsOverride', { width: 1440, height, deviceScaleFactor: 2, mobile: false })
  await page('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: scheme }] })
  const loaded = new Promise((resolve) => {
    const l = (msg) => msg.method === 'Page.loadEventFired' && msg.sessionId === sessionId && resolve()
    listeners.push(l)
  })
  await page('Page.navigate', { url: BASE + path })
  await loaded
  await sleep(2500) // let queries return and charts render
  if (act) {
    const { exceptionDetails } = await page('Runtime.evaluate', { expression: `(${act})()`, awaitPromise: true })
    if (exceptionDetails) throw new Error(`${name}: ${exceptionDetails.exception?.description ?? exceptionDetails.text}`)
    await sleep(800)
  }
  const { data } = await page('Page.captureScreenshot', { format: 'png' })
  writeFileSync(join(OUT, `${name}.png`), Buffer.from(data, 'base64'))
  console.log(`  ${OUT}/${name}.png`)
}

console.log('Writing screenshots:')
try {
  await shot('home', `/?${RANGE}`, 820)
  await shot('home-light', `/?${RANGE}`, 820, { scheme: 'light' })
  await shot('issues', `/issues?${RANGE}&issue=${issue.id}`, 620)
  await shot('compare', `/traces?${RANGE}&q=service:payment&tab=compare`, 700, {
    act: () => {
      const values = [...document.querySelectorAll('.compare .val')]
      ;(values.find((v) => v.textContent.trim() === 'acme-bank') ?? values[0])?.click()
    },
  })
  await shot('palette', `/?${RANGE}`, 620, {
    act: () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true })),
  })
  await shot('services', `/services?${RANGE}`, 620)
  await shot('service', `/services/checkout?${RANGE}`, 760)
  await shot('trace', `/traces/${trace.trace_id}`, 760)
  await shot('logs', `/logs?${RANGE}`, 760)
  await shot('metrics', `/metrics?${RANGE}&metric=http.server.request.duration&agg=p95&by=service.name`, 620)
  await shot('alerts', '/alerts', 620)
} finally {
  ws.close()
  const exited = new Promise((resolve) => chrome.once('exit', resolve))
  chrome.kill()
  await exited // Chrome writes to its profile until it exits
  rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 })
}
