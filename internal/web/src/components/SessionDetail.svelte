<script lang="ts">
  import { selectedSession, removeSession } from "../lib/stores/sessions";
  import StepTrace from "./StepTrace.svelte";

  function handleDelete() {
    const s = $selectedSession;
    if (!s) return;
    if (confirm(`Delete session "${s.id}"?`)) {
      removeSession(s.id);
    }
  }
</script>

{#if $selectedSession}
  {@const s = $selectedSession}

  <header class="detail-header">
    <span class="detail-title">
      {s.id}
    </span>
    <div class="detail-actions">
      <button
        class="icon-btn"
        title="Delete session"
        aria-label="Delete session"
        onclick={handleDelete}
      >
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <polyline points="3 6 5 6 21 6" />
          <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
          <path d="M10 11v6" />
          <path d="M14 11v6" />
          <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" />
        </svg>
      </button>
    </div>
  </header>

  <div class="session-detail">
    <div class="session-detail-inner">
      <StepTrace records={s.records || []} />
    </div>
  </div>
{/if}

<style>
  .detail-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-3) var(--space-5);
    border-bottom: 1px solid var(--color-border-subtle);
    background: var(--color-surface-base);
    flex-shrink: 0;
  }

  .detail-title {
    font-family: var(--font-family-code);
    font-size: 0.92rem;
    font-weight: 500;
    /* inherits --color-text-body — session IDs sit at chrome tier, not heading */
    letter-spacing: 0;
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  .detail-actions {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  /* Marketing-container: caps thread reading width, à la Modal blog layout */
  .session-detail {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-6) var(--space-5);
  }

  .session-detail-inner {
    max-width: 800px;
    margin: 0 auto;
    width: 100%;
  }

  @media (max-width: 900px) {
    .session-detail {
      padding: var(--space-4);
    }
  }
</style>
