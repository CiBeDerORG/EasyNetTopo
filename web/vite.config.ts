import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { cpSync, existsSync, readFileSync } from 'node:fs'
import { extname, join, resolve, sep } from 'node:path'

// Cesium Worker、资源和小部件与 EasyNetTopo 同源部署，避免跨域 Worker 重定向。
function cesiumAssets(): Plugin {
  const source = resolve(process.cwd(), 'node_modules/cesium/Build/Cesium')
  return {
    name: 'cesium-same-origin-assets',
    configureServer(server) {
      server.middlewares.use('/cesium/', (req, res, next) => {
        let target: string
        try {
          const pathname = decodeURIComponent((req.url || '').split('?')[0])
          target = resolve(source, '.' + sep + pathname.replace(/^\/+/, ''))
        } catch { res.statusCode = 400; res.end('Invalid path'); return }
        if (!target.startsWith(source + sep) || !existsSync(target)) { next(); return }
        const mime = new Map([
          ['.js', 'application/javascript'], ['.css', 'text/css'],
          ['.json', 'application/json'], ['.wasm', 'application/wasm'],
          ['.png', 'image/png'], ['.jpg', 'image/jpeg'],
          ['.jpeg', 'image/jpeg'], ['.svg', 'image/svg+xml']
        ])
        res.setHeader('Content-Type', mime.get(extname(target)) || 'application/octet-stream')
        try { res.end(readFileSync(target)) } catch { next() }
      })
    },
    closeBundle() {
      const output = resolve(process.cwd(), 'dist/cesium')
      for (const dir of ['Workers', 'Assets', 'ThirdParty', 'Widgets']) {
        const from = join(source, dir)
        if (!existsSync(from)) throw new Error('缺少 Cesium 构建资源：' + from)
        cpSync(from, join(output, dir), { recursive: true })
      }
    }
  }
}

export default defineConfig({
  plugins: [vue(), cesiumAssets()],
  define: { CESIUM_BASE_URL: JSON.stringify('/cesium/') },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080'
    }
  }
})
