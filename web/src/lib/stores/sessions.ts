import { writable } from 'svelte/store'
import type { Session } from '../types'
import { listSessions, getSession, deleteSession} from '../api'

export const sessions = writable<Session[]>([])
export const selectedId = writable<string | null>(null)
export const selectedSession = writable<Session | null>(null)

export async function refreshSessions(): Promise<void> {
  try {
    const data = await listSessions()
    sessions.set(data)
  } catch (e) {
    console.error('Failed to refresh sessions:', e)
  }
}

export async function selectSession(id: string): Promise<void> {
  selectedId.set(id)
  try {
    const data = await getSession(id)
    selectedSession.set(data)
  } catch (e) {
    console.error('Failed to fetch session detail:', e)
    selectedSession.set(null)
  }
}

export async function removeSession(id: string): Promise<void> {
  try {
    await deleteSession(id)
    sessions.update((list) => list.filter((s) => s.id !== id))
    selectedId.update((current) => (current === id ? null : current))
    selectedSession.update((current) => (current?.id === id ? null : current))
  } catch (e) {
    console.error('Failed to delete session:', e)
  }
}
