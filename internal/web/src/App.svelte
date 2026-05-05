<script lang="ts">
  import SessionList from "./components/SessionList.svelte";
  import SessionDetail from "./components/SessionDetail.svelte";
  import { refreshSessions, selectedSession } from "./lib/stores/sessions";

  $effect(() => {
    refreshSessions();
  });
</script>

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
</div>

<style>
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
    /* inherits --color-text-body */
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
</style>
