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
    <span class="session-avatar" aria-hidden="true">YO</span>
  </div>
</header>

<div class="session-search">
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
        class="session-row"
        aria-current={$selectedId === s.id ? "page" : undefined}
        onclick={() => selectSession(s.id)}
      >
        <div class="session-row-title">{s.id}</div>
        <div class="session-row-meta">
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
    padding: var(--space-4) var(--space-4) var(--space-3);
    flex-shrink: 0;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    font-family: var(--font-family-display);
    font-size: 1.18rem;
    font-weight: 500;
    letter-spacing: -0.015em;
    line-height: 1;
    color: var(--color-text-heading);
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

  .session-avatar {
    width: var(--size-avatar-sidebar);
    height: var(--size-avatar-sidebar);
    border-radius: var(--radius-full);
    background: linear-gradient(to top, var(--blue-600), var(--blue-500));
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--white);
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.02em;
    border: 1px solid var(--blue-600);
  }

  .session-search {
    margin: var(--space-3) var(--space-4) var(--space-2);
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px 0;
    background: transparent;
    color: var(--color-text-tertiary);
    transition: color var(--motion-duration-fast) var(--motion-ease);

    &:focus-within {
      color: var(--color-text-secondary);
    }

    & input {
      flex: 1;
      border: none;
      outline: none;
      background: transparent;
      font-family: var(--font-family-ui);
      font-size: 0.82rem;
      color: var(--color-text-body);

      &::placeholder {
        color: var(--color-text-tertiary);
      }
    }

    & svg {
      width: 14px;
      height: 14px;
      color: currentColor;
    }
  }

  .session-list {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-1) var(--space-2) var(--space-3);
  }

  .session-row {
    cursor: pointer;
    position: relative;
    margin: var(--space-1) 0;
    padding: 10px var(--space-3);
    border-radius: var(--radius-lg);
    transition:
      background var(--motion-duration-fast) var(--motion-ease),
      border-color var(--motion-duration-fast) var(--motion-ease),
      transform var(--motion-duration-fast) var(--motion-ease);
    display: block;
    border: 1px solid transparent;
    background: transparent;
    text-align: left;
    width: 100%;
    color: var(--color-text-body);

    &:hover {
      background: var(--color-row-hover);
      border-color: var(--color-border-subtle);
      transform: translateY(-1px);
    }

    &[aria-current="page"] {
      background: var(--color-action-primary-soft);
      border-color: var(--blue-100);
      box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.06);
    }
  }

  .session-row-title {
    font-size: 0.84rem;
    font-weight: 600;
    line-height: 1.3;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .session-row-meta {
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
    border: 1px dashed var(--color-border-subtle);
    border-radius: var(--radius-lg);
    margin: var(--space-2) 0;
  }
</style>
