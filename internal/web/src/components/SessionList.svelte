<script lang="ts">
  import {
    sessions,
    selectedId,
    selectSession,
    refreshSessions,
  } from "../lib/stores/sessions";

  let query = $state("");
  let refreshing = $state(false);

  const filtered = $derived(
    $sessions.filter((s) => {
      if (!query.trim()) return true;
      const q = query.toLowerCase();
      return (s.id).toLowerCase().includes(q);
    }),
  );

  async function handleRefresh() {
    refreshing = true;
    try {
      await refreshSessions();
    } finally {
      refreshing = false;
    }
  }

  function formatRelative(iso?: string): string {
    if (!iso) return "";
    const t = new Date(iso).getTime();
    if (isNaN(t)) return "";
    const diffSec = Math.floor((Date.now() - t) / 1000);
    if (diffSec < 60) return "just now";
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`;
    return `${Math.floor(diffSec / 86400)}d`;
  }
</script>

<header class="sidebar-header">
  <span class="brand">
    <img src="/icon-192.png" alt="" class="brand-mark" width="22" height="22" />
    Tinker
  </span>
  <div class="header-actions">
    <button
      class="icon-btn"
      title="Refresh"
      disabled={refreshing}
      onclick={handleRefresh}
      aria-label="Refresh sessions"
    >
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        class={refreshing ? "is-spinning" : ""}
      >
        <path d="M21 12a9 9 0 1 1-3-6.7L21 8" />
        <path d="M21 3v5h-5" />
      </svg>
    </button>
    <span class="avatar" aria-hidden="true">YO</span>
  </div>
</header>

<div class="search">
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
  >
    <circle cx="11" cy="11" r="7" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </svg>
  <input bind:value={query} placeholder="Search sessions..." />
</div>

<div class="session-list">
  {#if filtered.length === 0}
    <div class="session-empty">
      {query ? "No matches" : "No sessions yet"}
    </div>
  {:else}
    {#each filtered as s (s.id)}
      <button
        type="button"
        class="row {$selectedId === s.id ? 'active' : ''}"
        onclick={() => selectSession(s.id)}
      >
        <div class="row-title">{s.id}</div>
        <div class="row-meta">
          {formatRelative(s.start_time)}
          {#if s.context_count}
            · {s.context_count} context{s.context_count === 1 ? "" : "s"}
          {/if}
        </div>
      </button>
    {/each}
  {/if}
</div>

<style>
  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-4) var(--space-2);
    flex-shrink: 0;
  }

  /* Brand mark + wordmark — sleek, no pill chrome.
     Icon does the work; text inherits body color (light-green). */
  .brand {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-family: var(--font-family-display);
    font-size: 0.98rem;
    font-weight: 600;
    letter-spacing: -0.015em;
    line-height: 1;
  }

  .brand-mark {
    width: 22px;
    height: 22px;
    border-radius: var(--radius-md);
    display: block;
  }

  .header-actions {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  .avatar {
    width: 28px;
    height: 28px;
    border-radius: var(--radius-full);
    background: var(--color-surface-base);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-primary);
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    border: 1px solid var(--color-border-subtle);
  }

  .search {
    margin: var(--space-1) var(--space-3) var(--space-3);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 8px var(--space-3);
    border-radius: var(--radius-lg);
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid transparent;
    transition:
      border-color var(--motion-duration-fast) var(--motion-ease),
      background var(--motion-duration-fast) var(--motion-ease);

    &:focus-within {
      background: rgba(255, 255, 255, 0.10);
      border-color: var(--color-border-subtle);
    }

    & input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      font-family: var(--font-family-ui);
      font-size: 0.82rem;
      /* inherits --color-text-body */

      &::placeholder {
        color: var(--color-text-tertiary);
      }
    }

    & svg {
      width: 14px;
      height: 14px;
      color: var(--color-text-tertiary);
    }
  }

  /* Forward-looking: section labels for future MCP / Settings sections */
  :global(.section-label) {
    font-size: 0.66rem;
    font-weight: 600;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--color-text-tertiary);
    padding: var(--space-4) var(--space-4) var(--space-1);
  }

  .session-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-1) 0;
  }

  .row {
    cursor: pointer;
    position: relative;
    margin: 1px var(--space-2);
    padding: 8px var(--space-3) 8px 14px;
    border-radius: var(--radius-md);
    transition: background var(--motion-duration-fast) var(--motion-ease);
    display: block;
    border: none;
    background: transparent;
    text-align: left;
    width: calc(100% - var(--space-4));
    /* inherits --color-text-body */

    &:hover {
      background: var(--color-row-hover);
    }

    /* Active row: 2 px lime left bar instead of fully painting the row */
    &.active {
      background: var(--color-row-hover);

      &::before {
        content: "";
        position: absolute;
        left: 4px;
        top: 8px;
        bottom: 8px;
        width: 2px;
        background: var(--color-action-primary);
        border-radius: var(--radius-sm);
      }
    }
  }

  .row-title {
    font-size: 0.84rem;
    font-weight: 500;
    /* distinction from row-meta is weight, not hue */
    line-height: 1.3;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .row-meta {
    font-size: 0.72rem;
    color: var(--color-text-secondary);
    margin-top: 2px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .session-empty {
    text-align: center;
    color: var(--color-text-secondary);
    font-size: 0.78rem;
    padding: var(--space-8) var(--space-4);
  }
</style>
