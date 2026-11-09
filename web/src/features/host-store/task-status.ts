import type { ExecutionTask } from './types'

const inProgress = (task: ExecutionTask) =>
  task.state === 'running' || task.state === 'preparing'
const pending = (task: ExecutionTask) => task.state === 'queued'

export function currentHostTask(tasks: ExecutionTask[], hostId: string): ExecutionTask | undefined {
  const hostTasks = tasks.filter(task => task.host_id === hostId && task.type === 'host')
  return hostTasks.find(inProgress) || hostTasks.find(task => task.state === 'interrupted') || hostTasks.find(pending) || hostTasks[0]
}

export function currentAppTask(
  tasks: ExecutionTask[], hostId: string, sourceId: string, appId: string
): ExecutionTask | undefined {
  const appTasks = tasks.filter(task =>
    task.type === 'application' && task.host_id === hostId &&
    task.source_id === sourceId && task.app_id === appId
  )
  return appTasks.find(inProgress) || appTasks.find(task => task.state === 'interrupted') || appTasks.find(pending) || appTasks[0]
}

export function taskStateLabel(state: ExecutionTask['state']): string {
  return ({
    queued: '排队中',
    preparing: '准备中',
    running: '执行中',
    succeeded: '完成',
    failed: '执行失败',
    interrupted: '任务中断',
    cancelled: '已被新任务替代'
  } as Record<ExecutionTask['state'], string>)[state]
}

export function taskSourceLabel(source: string): string {
  return ({
    github_actions: 'GitHub Actions',
    manual: '手动更新',
    api: '接口触发'
  } as Record<string, string>)[source] || source
}

export function taskActive(task?: ExecutionTask): boolean {
  return !!task && (inProgress(task) || pending(task))
}

export function formatTaskTime(task: ExecutionTask): string {
  return new Date(task.finished_at || task.started_at || task.created_at).toLocaleString()
}

// 任务触发时间恒取创建时间，不能使用开始/完成时间替代。
export function formatTaskTriggerTime(task: ExecutionTask): string {
  const time = new Date(task.created_at)
  return Number.isNaN(time.getTime()) ? '未知' : time.toLocaleString()
}
