<script lang="ts">
  import SessionList from "./components/SessionList.svelte";
  import SessionDetail from "./components/SessionDetail.svelte";
  import Login from "./components/Login.svelte";
  import { refreshSessions, selectedSession } from "./lib/stores/sessions";
  import { auth, refreshMe, logout } from "./lib/auth";

  $effect(() => {
    refreshMe();
  });

  // Refresh sessions whenever the user becomes authenticated (or auth is disabled).
  let lastLoadedFor = $state<string | null>(null);
  $effect(() => {
    const state = $auth;
    if (!state.loaded) return;
    const key = state.authDisabled ? "anon" : (state.user?.email ?? null);
    if (key && key !== lastLoadedFor) {
      lastLoadedFor = key;
      refreshSessions();
    }
  });
</script>

{#if !$auth.loaded}
  <div class="boot">Loading…</div>
{:else if !$auth.authDisabled && !$auth.user}
  <Login />
{:else}
  <div class="min-h-screen flex flex-col">
    <main class="flex-1 min-h-0 flex">
      <div class="app-shell">
        <aside class="sidebar-pane">
          <SessionList />
        </aside>

        <section class="detail-pane">
          {#if $selectedSession}
            <SessionDetail />
          {:else}
            <div class="empty-state">
              <div class="empty-card">Ask or build anything</div>
            </div>
          {/if}
        </section>
      </div>
    </main>

    {#if $auth.user}
      <div class="user-chip">
        {#if $auth.user.picture}
          <img src={$auth.user.picture} alt="" />
        {/if}
        <span class="user-email">{$auth.user.email}</span>
        <button type="button" onclick={() => logout()}>Sign out</button>
      </div>
    {/if}
  </div>
{/if}

<style>
  .boot {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--font-family-display);
    color: var(--color-text-body);
  }

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-8);
  }

  .empty-card {
    width: min(520px, 90%);
    background: var(--color-surface-base);
    border: 1px solid var(--color-border-accent);
    border-radius: var(--radius-xl);
    padding: var(--space-6);
    display: flex;
    align-items: center;
    gap: var(--space-3);
    font-family: var(--font-family-display);
    font-size: 0.92rem;

    &::before {
      content: "";
      width: 8px;
      height: 8px;
      border-radius: var(--radius-full);
      background: var(--color-action-primary);
      box-shadow: 0 0 0 3px var(--green-30);
      flex-shrink: 0;
    }
  }

  .user-chip {
    position: fixed;
    bottom: var(--space-3);
    right: var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--color-surface-base);
    border: 1px solid var(--color-border-accent);
    border-radius: var(--radius-full);
    padding: var(--space-1) var(--space-3);
    font-family: var(--font-family-display);
    font-size: 0.78rem;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }

  .user-chip img {
    width: 24px;
    height: 24px;
    border-radius: 50%;
  }

  .user-chip button {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-text-body);
    text-decoration: underline;
    font-size: 0.78rem;
  }

  .user-email {
    color: var(--color-text-body);
  }
</style>
