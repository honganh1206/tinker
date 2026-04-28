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
      return (s.name || s.id).toLowerCase().includes(q);
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
    <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect width="40" height="40" rx="8" fill="#1A4D55" />
      <circle cx="20" cy="20" r="11" fill="#FCF3ED" />
      <circle cx="25" cy="18" r="3" fill="#1A4D55" />
    </svg>
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
        <div class="row-title">{s.name || s.id}</div>
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
