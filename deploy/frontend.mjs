import { createRequire } from 'node:module'
import { isIP } from 'node:net'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const WORKSPACE_ROOT = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const API_METADATA = "window.apiMeta = { api_host: '' };\n"

function parsePort(value, name, fallback) {
  const text = String(value ?? '').trim()
  if (!text && fallback !== undefined) return fallback
  if (!/^\d+$/.test(text) || Number(text) < 1 || Number(text) > 65535) {
    throw new Error(`${name} must be an integer between 1 and 65535`)
  }
  return Number(text)
}

function parseHostname(value, name, allowSubdomains = false) {
  const hostname = value.trim()
  const candidate = allowSubdomains && hostname.startsWith('.') ? hostname.slice(1) : hostname
  if (isIP(candidate.replace(/^\[|\]$/g, ''))) return hostname
  if (!candidate || candidate === 'true' || candidate === 'false' ||
      !/^[a-z\d](?:[a-z\d.-]*[a-z\d])?$/i.test(candidate) || candidate.includes('..')) {
    throw new Error(`${name} must contain hostnames, without a scheme, port, path, or wildcard`)
  }
  return hostname.toLowerCase()
}

/** Read only the deployment variables. Repository .env files are never loaded by Vite. */
export function readFrontendEnvironment(env = process.env) {
  let apiUrl
  try {
    apiUrl = new URL(env.API_BASE_URL || 'http://backend:11211')
  } catch {
    throw new Error('API_BASE_URL must be an HTTP or HTTPS origin')
  }
  if (!['http:', 'https:'].includes(apiUrl.protocol) || apiUrl.username || apiUrl.password ||
      apiUrl.pathname !== '/' || apiUrl.search || apiUrl.hash) {
    throw new Error('API_BASE_URL must be an HTTP or HTTPS origin without credentials or a path')
  }

  const allowedHosts = [...new Set(String(env.VITE_ALLOWED_HOSTS || '').split(',')
    .map(hostname => hostname.trim()).filter(Boolean)
    .map(hostname => parseHostname(hostname, 'VITE_ALLOWED_HOSTS', true)))]
  const hmr = {}
  if (env.VITE_HMR_HOST?.trim()) hmr.host = parseHostname(env.VITE_HMR_HOST, 'VITE_HMR_HOST')
  if (env.VITE_HMR_PORT?.trim()) hmr.clientPort = parsePort(env.VITE_HMR_PORT, 'VITE_HMR_PORT')
  if (env.VITE_HMR_PROTOCOL?.trim()) {
    const protocol = env.VITE_HMR_PROTOCOL.trim()
    if (!['ws', 'wss'].includes(protocol)) throw new Error('VITE_HMR_PROTOCOL must be ws or wss')
    hmr.protocol = protocol
  }

  return {
    webPort: parsePort(env.WEB_PORT, 'WEB_PORT', 11210),
    apiOrigin: apiUrl.origin,
    dataDir: resolve(env.EASYTIER_DATA_DIR || '/data'),
    allowedHosts,
    hmr: Object.keys(hmr).length ? hmr : undefined,
  }
}

/** Override the checked-in public metadata before Vite's static-file middleware. */
export function apiMetadataPlugin() {
  return {
    name: 'easytier-development-api-metadata',
    configureServer(server) {
      server.middlewares.use((request, response, next) => {
        let pathname
        try {
          pathname = decodeURIComponent((request.url || '').split('?')[0])
        } catch {
          next()
          return
        }
        if (pathname !== '/api_meta.js') {
          next()
          return
        }
        if (request.method !== 'GET' && request.method !== 'HEAD') {
          response.writeHead(405, { Allow: 'GET, HEAD' })
          response.end()
          return
        }
        response.writeHead(200, {
          'Content-Type': 'application/javascript; charset=utf-8',
          'Cache-Control': 'no-store',
          'X-Content-Type-Options': 'nosniff',
          'Content-Length': Buffer.byteLength(API_METADATA),
        })
        response.end(request.method === 'HEAD' ? undefined : API_METADATA)
      })
    },
  }
}

async function importDependency(requireFrom, name) {
  const namespace = await import(pathToFileURL(requireFrom.resolve(name)).href)
  // Some CommonJS plugins expose an ES-module default inside module.exports.
  return namespace.default?.default ?? namespace.default ?? namespace
}

async function loadVite(requireFrom) {
  const manifestPath = requireFrom.resolve('vite/package.json')
  const manifest = requireFrom(manifestPath)
  // Use Vite's published ESM entry instead of its deprecated CommonJS bridge.
  return import(pathToFileURL(resolve(dirname(manifestPath), manifest.module || manifest.main)).href)
}

/** Keep this development-only configuration outside the normal frontend build. */
export async function createFrontendConfig({ workspaceRoot = WORKSPACE_ROOT, env = process.env } = {}) {
  const settings = readFrontendEnvironment(env)
  const frontendRoot = resolve(workspaceRoot, 'easytier-web/frontend')
  const libraryRoot = resolve(workspaceRoot, 'easytier-web/frontend-lib')
  const frontendRequire = createRequire(resolve(frontendRoot, 'package.json'))
  const libraryRequire = createRequire(resolve(libraryRoot, 'package.json'))
  const [vue, yaml, tailwind, loadTailwindConfig, autoprefixer, nested] = await Promise.all([
    importDependency(frontendRequire, '@vitejs/plugin-vue'),
    importDependency(frontendRequire, '@modyfi/vite-plugin-yaml'),
    importDependency(frontendRequire, 'tailwindcss'),
    importDependency(frontendRequire, 'tailwindcss/loadConfig'),
    importDependency(frontendRequire, 'autoprefixer'),
    importDependency(libraryRequire, 'postcss-nested'),
  ])
  const tailwindConfig = loadTailwindConfig(resolve(frontendRoot, 'tailwind.config.js'))
  const cacheDir = resolve(settings.dataDir, 'cache/vite')

  return {
    configFile: false,
    envFile: false,
    envDir: resolve(settings.dataDir, 'run/vite-env'),
    root: frontendRoot,
    cacheDir,
    plugins: [apiMetadataPlugin(), vue(), yaml()],
    resolve: {
      alias: [
        { find: /^easytier-frontend-lib$/, replacement: resolve(libraryRoot, 'src/easytier-frontend-lib.ts') },
        { find: /^easytier-frontend-lib\/style\.css$/, replacement: resolve(libraryRoot, 'src/style.css') },
      ],
      dedupe: ['vue'],
    },
    optimizeDeps: { exclude: ['easytier-frontend-lib'] },
    css: {
      postcss: {
        plugins: [
          tailwind({
            ...tailwindConfig,
            content: [
              resolve(frontendRoot, 'index.html'),
              resolve(frontendRoot, 'src/**/*.{vue,js,ts,jsx,tsx}'),
              resolve(libraryRoot, 'src/**/*.{vue,js,ts,jsx,tsx}'),
            ],
          }),
          autoprefixer(),
          nested(),
        ],
      },
    },
    server: {
      host: '0.0.0.0',
      port: settings.webPort,
      strictPort: true,
      cors: false,
      allowedHosts: settings.allowedHosts,
      hmr: settings.hmr,
      fs: {
        strict: true,
        allow: [frontendRoot, libraryRoot, resolve(workspaceRoot, 'node_modules'), cacheDir],
      },
      proxy: {
        '^/api/': { target: settings.apiOrigin, changeOrigin: false, ws: true },
      },
    },
  }
}

export async function startFrontend(options = {}) {
  const workspaceRoot = options.workspaceRoot ?? WORKSPACE_ROOT
  const frontendRequire = createRequire(resolve(workspaceRoot, 'easytier-web/frontend/package.json'))
  const [{ createServer }, config] = await Promise.all([
    loadVite(frontendRequire),
    createFrontendConfig({ ...options, workspaceRoot }),
  ])
  const server = await createServer(config)
  try {
    await server.listen()
    return server
  } catch (error) {
    await server.close()
    throw error
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    const server = await startFrontend()
    let closing = false
    const shutdown = async () => {
      if (closing) return
      closing = true
      try {
        await server.close()
        process.exit(0)
      } catch {
        process.exit(1)
      }
    }
    process.once('SIGTERM', shutdown)
    process.once('SIGINT', shutdown)
    server.printUrls()
  } catch (error) {
    console.error(`[frontend] ${error.message}`)
    process.exitCode = 1
  }
}
