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
  <div class="app-root pattern">
    <main class="app-main">
      <div class="app-shell">
        <aside class="sidebar-pane">
          <SessionList />
        </aside>

        {#if $selectedSession}
          <section class="detail-pane">
            <SessionDetail />
          </section>
        {:else}
          <div class="empty-state">
            <div class="empty-card">
              <h1>Ask or build anything</h1>
              <p>
                Select a session from the paper stack, or start a new run from
                the CLI.
              </p>
            </div>
          </div>
        {/if}
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
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-xl);
    padding: var(--space-8);
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--space-2);
    box-shadow: var(--shadow-card);
  }

  .empty-card h1 {
    margin: 0;
    font-family: var(--font-family-display);
    font-size: clamp(2rem, 6vw, 3.2rem);
    font-weight: 500;
    line-height: 0.95;
    color: var(--color-text-heading);
  }

  .empty-card p {
    margin: var(--space-2) 0 0;
    color: var(--color-text-secondary);
    line-height: 1.6;
    max-width: 32rem;
  }

  .user-chip {
    position: fixed;
    bottom: var(--space-3);
    right: var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    background: var(--color-surface-base);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-full);
    padding: var(--space-1) var(--space-3);
    font-family: var(--font-family-ui);
    font-size: 0.78rem;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
  }

  .user-chip img {
    width: var(--size-avatar-sm);
    height: var(--size-avatar-sm);
    border-radius: 50%;
  }

  .user-chip button {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-text-secondary);
    text-decoration: underline;
    font-size: 0.78rem;
  }

  .user-email {
    color: var(--color-text-body);
  }
</style>
