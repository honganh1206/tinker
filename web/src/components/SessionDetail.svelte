<script lang="ts">
  import { selectedSession, removeSession } from "../lib/stores/sessions";
  import StepTrace from "./StepTrace.svelte";

  function handleDelete() {
    const s = $selectedSession;
    if (!s) return;
    if (confirm(`Delete session "${s.name || s.id}"?`)) {
      removeSession(s.id);
    }
  }
</script>

{#if $selectedSession}
  {@const s = $selectedSession}

  <header class="detail-header">
    <span class="detail-title">
      {s.contexts?.[0]?.name || s.name || s.id}
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
    <StepTrace records={s.records || []} />
  </div>
{/if}
