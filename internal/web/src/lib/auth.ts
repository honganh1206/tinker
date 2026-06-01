import { writable } from 'svelte/store'

export interface CurrentUser {
  email: string
  name: string
  picture: string
}

// AuthState describes whether the API server requires authentication and,
// if so, who is currently signed in.
export interface AuthState {
  // loaded is true once we have asked /auth/me at least once.
  loaded: boolean
  // authDisabled means the server was started without a Google client ID;
  // every request is anonymous.
  authDisabled: boolean
  // user is the signed-in user, or null when not signed in.
  user: CurrentUser | null
}

export const auth = writable<AuthState>({
  loaded: false,
  authDisabled: false,
  user: null,
})

// refreshMe fetches /auth/me and updates the auth store.
export async function refreshMe(): Promise<void> {
  try {
    const res = await fetch('/auth/me', { credentials: 'same-origin' })
    if (res.status === 401) {
      auth.set({ loaded: true, authDisabled: false, user: null })
      return
    }
    if (!res.ok) throw new Error(`/auth/me failed: ${res.status}`)
    const body = await res.json()
    if (body && body.authDisabled) {
      auth.set({ loaded: true, authDisabled: true, user: null })
      return
    }
    auth.set({
      loaded: true,
      authDisabled: false,
      user: {
        email: body.email,
        name: body.name,
        picture: body.picture,
      },
    })
  } catch (e) {
    console.error('Failed to load /auth/me:', e)
    auth.set({ loaded: true, authDisabled: false, user: null })
  }
}

export async function logout(): Promise<void> {
  await fetch('/auth/logout', {
    method: 'POST',
    credentials: 'same-origin',
  })
  auth.set({ loaded: true, authDisabled: false, user: null })
}
