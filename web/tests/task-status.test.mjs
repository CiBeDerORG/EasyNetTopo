import assert from 'node:assert/strict'
import test from 'node:test'
import { formatTaskTime, formatTaskTriggerTime, taskActive } from '../src/features/host-store/task-status.ts'

function makeTask(state, overrides = {}) {
  return {
    id: 'task-1',
    type: 'application',
    host_id: 'host-1',
    source_id: 'source-1',
    app_id: 'app-1',
    action: 'update',
    trigger_source: 'github_actions',
    state,
    created_at: '2026-11-09T06:30:00.000Z',
    started_at: '2026-11-09T06:31:00.000Z',
    finished_at: '2026-11-09T06:32:00.000Z',
    ...overrides
  }
}

test('排队、准备和执行中的任务使用触发时间展示模式', () => {
  for (const state of ['queued', 'preparing', 'running']) {
    assert.equal(taskActive(makeTask(state)), true, state)
  }
})

test('所有终态任务使用最近任务时间展示模式', () => {
  for (const state of ['succeeded', 'failed', 'interrupted', 'cancelled']) {
    assert.equal(taskActive(makeTask(state)), false, state)
  }
  assert.equal(taskActive(undefined), false)
})

test('最近任务时间优先使用完成时间，其次开始时间，最后回退到创建时间', () => {
  const finished = makeTask('succeeded')
  assert.equal(formatTaskTime(finished), new Date(finished.finished_at).toLocaleString())

  const started = makeTask('failed', { finished_at: undefined })
  assert.equal(formatTaskTime(started), new Date(started.started_at).toLocaleString())

  const cancelled = makeTask('cancelled', { started_at: undefined, finished_at: undefined })
  assert.equal(formatTaskTime(cancelled), new Date(cancelled.created_at).toLocaleString())
})

test('触发时间始终取创建时间，不受开始和完成时间影响', () => {
  const task = makeTask('succeeded')
  assert.equal(formatTaskTriggerTime(task), new Date(task.created_at).toLocaleString())
})
