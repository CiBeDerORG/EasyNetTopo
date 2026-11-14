import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const readSource = path => readFileSync(new URL(path, import.meta.url), 'utf8')

const hostStorePage = readSource('../src/features/host-store/views/HostStorePage.vue')
const networkMapPane = readSource('../src/features/network-map/NetworkMapPane.vue')
const globeMap = readSource('../src/features/network-map/GlobeMap.vue')

test('Cesium network map is excluded from the initial workbench bundle', () => {
  assert.match(hostStorePage, /loader:\s*\(\)\s*=>\s*import\(['"]\.\.\/\.\.\/network-map\/NetworkMapPane\.vue['"]\)/)
  assert.doesNotMatch(hostStorePage, /import\s+NetworkMapPane\s+from\s+['"]\.\.\/\.\.\/network-map\/NetworkMapPane\.vue['"]/)
})

test('network map data and controls load before the asynchronous Cesium globe', () => {
  assert.match(networkMapPane, /loader:\s*\(\)\s*=>\s*import\(['"]\.\/GlobeMap\.vue['"]\)/)
  assert.doesNotMatch(networkMapPane, /import\s+GlobeMap\s+from\s+['"]\.\/GlobeMap\.vue['"]/)
  assert.match(networkMapPane, /globe-loading/)
})

test('WebGL viewer initialization is deferred until the browser is idle', () => {
  assert.match(globeMap, /requestIdleCallback/)
  assert.match(globeMap, /timeout:\s*1200/)
  assert.match(globeMap, /cancelInitialization\?\.\(\)/)
})
