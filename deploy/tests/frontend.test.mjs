import assert from 'node:assert/strict'
import { once } from 'node:events'
import { readFile, mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import http from 'node:http'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'
import { test } from 'node:test'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { runInNewContext } from 'node:vm'

import { apiMetadataPlugin, createFrontendConfig, readFrontendEnvironment } from '../frontend.mjs'

const REPOSITORY_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

async function listen(server) {
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  return `http://127.0.0.1:${server.address().port}`
}

function request(url, { method = 'GET', headers = {} } = {}) {
  return new Promise((resolveRequest, reject) => {
    const outgoing = http.request(url, { method, headers, agent: false }, incoming => {
      const chunks = []
      incoming.on('data', chunk => chunks.push(chunk))
      incoming.on('end', () => resolveRequest({
        status: incoming.statusCode,
        headers: incoming.headers,
        body: Buffer.concat(chunks).toString(),
      }))
      incoming.on('error', reject)
    })
    outgoing.on('error', reject)
    outgoing.end()
  })
}

function close(server) {
  return new Promise((resolveClose, reject) => server.close(error => error ? reject(error) : resolveClose()))
}

function waitForMessage(socket, predicate) {
  return new Promise((resolveMessage, reject) => {
    const timeout = setTimeout(() => finish(new Error('Timed out waiting for Vite HMR')), 15000)
    const onError = () => finish(new Error('Vite HMR WebSocket failed'))
    const onMessage = event => {
      const message = JSON.parse(event.data)
      if (predicate(message)) finish(undefined, message)
    }
    const finish = (error, message) => {
      clearTimeout(timeout)
      socket.removeEventListener('error', onError)
      socket.removeEventListener('message', onMessage)
      if (error) reject(error)
      else resolveMessage(message)
    }
    socket.addEventListener('error', onError)
    socket.addEventListener('message', onMessage)
  })
}

test('deployment environment keeps API private and HMR on the existing listener', () => {
  const defaults = readFrontendEnvironment({})
  assert.equal(defaults.webPort, 11210)
  assert.equal(defaults.apiOrigin, 'http://backend:11211')
  assert.equal(defaults.dataDir, '/data')
  assert.deepEqual(defaults.allowedHosts, [])
  assert.equal(defaults.hmr, undefined)

  const settings = readFrontendEnvironment({
    WEB_PORT: '11220',
    API_BASE_URL: 'http://backend:11221/',
    EASYTIER_DATA_DIR: '/tmp/easytier-frontend-test-data',
    VITE_HMR_HOST: 'Dev.Example.test',
    VITE_HMR_PORT: '443',
    VITE_HMR_PROTOCOL: 'wss',
    VITE_ALLOWED_HOSTS: 'dev.example.test, .example.test,dev.example.test',
  })
  assert.equal(settings.apiOrigin, 'http://backend:11221')
  assert.equal(settings.webPort, 11220)
  assert.equal(settings.dataDir, '/tmp/easytier-frontend-test-data')
  assert.deepEqual(settings.hmr, { host: 'dev.example.test', clientPort: 443, protocol: 'wss' })
  assert.deepEqual(settings.allowedHosts, ['dev.example.test', '.example.test'])
})

test('invalid deployment values fail without echoing credentials', () => {
  for (const env of [
    { WEB_PORT: '0' }, { WEB_PORT: '65536' }, { WEB_PORT: '11210bad' },
    { VITE_HMR_PORT: '-1' }, { VITE_HMR_PROTOCOL: 'https' },
    { VITE_HMR_HOST: 'https://dev.example.test' },
    { VITE_ALLOWED_HOSTS: 'true' }, { VITE_ALLOWED_HOSTS: '*' },
    { VITE_ALLOWED_HOSTS: 'dev.example.test:11210' },
    { API_BASE_URL: 'file:///tmp/example' },
    { API_BASE_URL: 'http://backend:11211/api/' },
    { API_BASE_URL: 'http://user:fictional-test-secret@backend:11211' },
  ]) {
    assert.throws(() => readFrontendEnvironment(env), error => {
      assert.doesNotMatch(error.message, /fictional-test-secret/)
      return true
    })
  }
})

test('API metadata always selects the browser origin and bypasses the backend', async t => {
  let middleware
  apiMetadataPlugin().configureServer({ middlewares: { use: handler => { middleware = handler } } })
  const server = http.createServer((incoming, outgoing) => {
    middleware(incoming, outgoing, () => { outgoing.writeHead(404); outgoing.end('unhandled') })
  })
  const origin = await listen(server)
  t.after(() => close(server))
  for (const path of ['/api_meta.js', '/api_meta.js?version=1', '/api%5fmeta.js']) {
    const result = await request(origin + path)
    assert.equal(result.status, 200)
    assert.match(result.headers['content-type'], /javascript/)
    assert.equal(result.headers['cache-control'], 'no-store')
    const context = { window: {} }
    runInNewContext(result.body, context)
    assert.equal(context.window.apiMeta.api_host, '')
    assert.doesNotMatch(result.body, /backend|config-server\.easytier\.cn/)
  }
  assert.equal((await request(origin + '/api_meta.js', { method: 'HEAD' })).body, '')
  const wrongMethod = await request(origin + '/api_meta.js', { method: 'POST' })
  assert.equal(wrongMethod.status, 405)
  assert.equal(wrongMethod.headers.allow, 'GET, HEAD')
  assert.equal((await request(origin + '/api/v1/test')).status, 404)
  assert.equal((await request(origin + '/api_meta.js/other')).status, 404)
})

// Opt-in integration needs a disposable source copy with pnpm install and Proto codegen
// already completed. It never installs dependencies or mutates the original checkout.
const integrationWorkspace = process.env.EASYTIER_FRONTEND_TEST_WORKSPACE
test('real Vite serves shared source, proxies same-origin API and sends HMR', {
  skip: !integrationWorkspace && 'Set EASYTIER_FRONTEND_TEST_WORKSPACE to a prepared disposable workspace',
  timeout: 90000,
}, async t => {
  const cleanups = []
  t.after(async () => {
    for (const cleanup of cleanups.reverse()) await cleanup()
  })
  const workspaceRoot = resolve(integrationWorkspace)
  assert.notEqual(workspaceRoot, REPOSITORY_ROOT, 'Use a disposable source copy for integration tests')
  const frontendRoot = resolve(workspaceRoot, 'easytier-web/frontend')
  const libraryRoot = resolve(workspaceRoot, 'easytier-web/frontend-lib')
  await mkdir(resolve(workspaceRoot, 'data'), { recursive: true })
  const stateRoot = await mkdtemp(resolve(workspaceRoot, 'data/frontend-test-'))
  cleanups.push(() => rm(stateRoot, { recursive: true, force: true }))
  const probeFile = resolve(libraryRoot, `src/ddp-hmr-probe-${process.pid}.vue`)
  const probeSource = marker => `<script setup>import { ref } from 'vue'; const label = ref('${marker}')</script>
    <template><div class="ddp-probe pt-[137px]">{{ label }}<span class="child" /></div></template>
    <style>.ddp-probe { & .child { color: red; } }</style>`
  await writeFile(probeFile, probeSource('ddp-before-edit'))
  cleanups.push(() => rm(probeFile, { force: true }))
  const envFile = resolve(frontendRoot, '.env')
  const previousEnv = await readFile(envFile).catch(error => {
    if (error.code !== 'ENOENT') throw error
    return undefined
  })
  await writeFile(envFile, 'VITE_DDP_ENV_PROBE=must-not-be-exposed\n')
  cleanups.push(() => previousEnv ? writeFile(envFile, previousEnv) : rm(envFile, { force: true }))

  const forwarded = []
  const backend = http.createServer((incoming, outgoing) => {
    forwarded.push({ path: incoming.url, host: incoming.headers.host, origin: incoming.headers.origin })
    outgoing.writeHead(200, { 'Content-Type': 'application/json' })
    outgoing.end(JSON.stringify(forwarded.at(-1)))
  })
  const apiOrigin = await listen(backend)
  cleanups.push(() => close(backend))
  const config = await createFrontendConfig({ workspaceRoot, env: {
    WEB_PORT: '11210', API_BASE_URL: apiOrigin, EASYTIER_DATA_DIR: stateRoot,
    VITE_ALLOWED_HOSTS: 'dev.easytier.test',
  } })
  const frontendRequire = createRequire(resolve(frontendRoot, 'package.json'))
  const viteManifest = frontendRequire.resolve('vite/package.json')
  const vitePackage = frontendRequire(viteManifest)
  const { createServer } = await import(pathToFileURL(resolve(dirname(viteManifest), vitePackage.module || vitePackage.main)).href)
  const server = await createServer({ ...config, logLevel: 'error', server: { ...config.server, host: '127.0.0.1', port: 0 } })
  cleanups.push(() => server.close())
  await server.listen()
  const origin = `http://127.0.0.1:${server.httpServer.address().port}`
  const fileUrl = filename => `/@fs/${filename}`

  await t.test('metadata stays local and proxy retains browser Host and Origin', async () => {
    const meta = await request(origin + '/api_meta.js')
    assert.equal(meta.status, 200)
    assert.doesNotMatch(meta.body, /backend|127\.0\.0\.1|config-server\.easytier\.cn/)
    assert.equal(forwarded.length, 0)
    const browserHost = 'dev.easytier.test:11210'
    const browserOrigin = `https://${browserHost}`
    const api = await request(origin + '/api/v1/auth/status?probe=1', {
      headers: { Host: browserHost, Origin: browserOrigin },
    })
    assert.equal(api.status, 200)
    assert.deepEqual(JSON.parse(api.body), { path: '/api/v1/auth/status?probe=1', host: browserHost, origin: browserOrigin })
    await request(origin + '/api')
    assert.equal(forwarded.length, 1, 'The proxy must only match /api/ paths')
  })

  await t.test('workspace secrets stay outside Vite env and file serving', async () => {
    assert.equal(server.config.env.VITE_DDP_ENV_PROBE, undefined)
    const privateFile = resolve(stateRoot, 'private-probe.txt')
    await writeFile(privateFile, 'fictional-private-test-value')
    const denied = await request(origin + fileUrl(privateFile))
    assert.equal(denied.status, 403)
    assert.doesNotMatch(denied.body, /fictional-private-test-value/)
    assert.equal((await request(origin + '/src/main.ts', { headers: { Host: 'unlisted.example.test' } })).status, 403)
  })

  await t.test('shared TypeScript, Vue, YAML, Proto and CSS compile directly', async () => {
    const main = await request(origin + '/src/main.ts')
    assert.equal(main.status, 200)
    assert.match(main.body, /frontend-lib\/src\/easytier-frontend-lib\.ts/)
    assert.doesNotMatch(main.body, /frontend-lib\/dist\//)
    const optimizedVue = main.body.match(/from\s+["']([^"']*\/vue\.js\?[^"']*)["']/)
    assert.ok(optimizedVue, 'The real Vue dependency must be optimized before teardown')
    assert.equal((await request(origin + optimizedVue[1])).status, 200)
    for (const path of [
      'src/easytier-frontend-lib.ts', 'src/components/UrlInput.vue',
      'src/locales/en.yaml', 'src/generated/proto/common.ts',
    ]) {
      const result = await request(origin + fileUrl(resolve(libraryRoot, path)))
      assert.equal(result.status, 200, `${path}: ${result.body.slice(0, 200)}`)
    }
    const css = await request(origin + fileUrl(resolve(libraryRoot, 'src/style.css')) + '?direct')
    assert.equal(css.status, 200)
    assert.match(css.body, /padding-top: 137px/, 'Tailwind must scan the shared library templates')
    const nestedCss = await request(origin + fileUrl(probeFile) + '?direct&vue&type=style&index=0&lang.css')
    assert.equal(nestedCss.status, 200)
    assert.match(nestedCss.body, /\.ddp-probe \.child/, 'Shared CSS must use postcss-nested')
    const resolveVue = importer => server.pluginContainer.resolveId('vue', importer)
    assert.equal((await resolveVue(resolve(frontendRoot, 'src/main.ts'))).id, (await resolveVue(probeFile)).id)
  })

  await t.test('shared Vue edits send a real WebSocket update without restarting Vite', async () => {
    const socket = new WebSocket(origin.replace(/^http/, 'ws'), 'vite-hmr')
    cleanups.push(() => socket.close())
    await waitForMessage(socket, message => message.type === 'connected')
    const before = await request(origin + fileUrl(probeFile))
    assert.equal(before.status, 200)
    assert.match(before.body, /ddp-before-edit/)
    const update = waitForMessage(socket, message => message.type === 'update' && message.updates.some(item => item.path.includes('ddp-hmr-probe-')))
    await writeFile(probeFile, probeSource('ddp-after-edit'))
    await update
    const after = await request(origin + fileUrl(probeFile))
    assert.equal(after.status, 200)
    assert.match(after.body, /ddp-after-edit/)
    socket.close()
  })
  await server.waitForRequestsIdle()
})
