import assert from 'node:assert/strict'
import test from 'node:test'
import { apiRequest, ApiError, formatApiError } from '../src/lib/api-client.ts'

test('配对结构化诊断保留中文说明、错误码及阶段，不泄漏 JSON 原文', async () => {
  const original = globalThis.fetch
  globalThis.fetch = async () => new Response(JSON.stringify({
    code: 'agent_unreachable',
    stage: 'identity',
    message: '无法连接 Agent；请检查服务器到 Agent 的网络'
  }), { status: 502, headers: { 'Content-Type': 'application/json; charset=utf-8' } })
  try {
    await assert.rejects(apiRequest('/api/v1/hosts/example/pair'), error => {
      assert.ok(error instanceof ApiError)
      assert.equal(error.status, 502)
      assert.equal(error.code, 'agent_unreachable')
      assert.equal(error.stage, 'identity')
      assert.match(formatApiError(error), /无法连接 Agent/)
      assert.match(formatApiError(error), /错误码：agent_unreachable/)
      assert.match(formatApiError(error), /身份核验（identity）/)
      assert.ok(!formatApiError(error).includes('{"code"'))
      return true
    })
  } finally { globalThis.fetch = original }
})

test('结构化业务错误保留友好 message 并优先于 JSON 原文', async () => {
  const original = globalThis.fetch
  globalThis.fetch = async () => new Response(
    JSON.stringify({ code: 'invalid_host_metadata', message: '主机名称或连接地址不合法' }),
    { status: 400, headers: { 'Content-Type': 'application/json' } }
  )
  try {
    await assert.rejects(apiRequest('/api/v1/hosts'), error => {
      assert.match(formatApiError(error), /主机名称或连接地址不合法/)
      assert.match(formatApiError(error), /invalid_host_metadata/)
      return true
    })
  } finally { globalThis.fetch = original }
})

test('普通文本错误兼容既有后端；无正文时显示 HTTP 状态', async () => {
  const original = globalThis.fetch
  try {
    globalThis.fetch = async () => new Response('登录尝试过多', { status: 429 })
    await assert.rejects(apiRequest('/api/v1/auth/login'), error => {
      assert.equal(formatApiError(error), '登录尝试过多')
      return true
    })
    globalThis.fetch = async () => new Response('', { status: 503 })
    await assert.rejects(apiRequest('/api/v1/auth/login'), error => {
      assert.match(formatApiError(error), /HTTP 503/)
      return true
    })
  } finally { globalThis.fetch = original }
})
