<script>
  import Header from './components/Header.svelte'
  import SessionList from './components/SessionList.svelte'
  import SessionDetail from './components/SessionDetail.svelte'
  import { refreshSessions, selectedId, selectedSession } from './lib/stores/sessions.js'

  $effect(() => {
    refreshSessions()
    const interval = setInterval(refreshSessions, 3000)
    return () => clearInterval(interval)
  })
</script>

<div theme="light" class="h-screen overflow-hidden grid grid-rows-[44px_1fr] grid-cols-[300px_1fr]">
  <div class="col-span-2">
    <Header />
  </div>
  <SessionList />
  <main class="flex flex-col overflow-hidden bg-[var(--color-bg-primary)]">
    {#if $selectedSession}
      <SessionDetail />
    {:else}
      <div class="text-[var(--color-text-tertiary)] text-[var(--font-size-sm)] p-16 text-center font-[var(--font-mono)]">
        ← select a session to inspect
      </div>
    {/if}
  </main>
</div>
