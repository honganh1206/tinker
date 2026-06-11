<script lang="ts">
  import {
    ChevronRight,
    ChevronDown,
    Wrench,
    CornerDownRight,
  } from "lucide-svelte";
  import { RecordType } from "../lib/types";
  import type { Record } from "../lib/types";
  import { renderMarkdown } from "../lib/markdown";

  let { records = [] }: { records: Record[] } = $props();

  let expanded: { [key: number]: boolean } = $state({});

  function toggleExpand(id: number): void {
    expanded[id] = !expanded[id];
  }

  function truncate(s: string, n: number): string {
    return s && s.length > n ? s.slice(0, n) + "…" : s;
  }
</script>

{#if records.length === 0}
  <div class="trace-empty">No records yet</div>
{:else}
  <div class="trace-list">
    {#each records as r (r.id)}
      {#if r.source === RecordType.Prompt}
        <!-- user: avatar on right, bubble on right -->
        <div class="message" data-author="user">
          <div class="message-avatar" aria-hidden="true">YO</div>
          <div class="message-bubble md">
            {@html renderMarkdown(r.content)}
          </div>
        </div>
      {:else if r.source === RecordType.ModelResp}
        <!-- agent: avatar on left, bubble on left -->
        <div class="message" data-author="agent">
          <div class="message-avatar" aria-hidden="true">
            <img src="/icon-192.png" alt="" width="32" height="32" />
          </div>
          <div class="message-bubble md">
            {@html renderMarkdown(r.content)}
          </div>
        </div>
      {:else if r.source === RecordType.ToolUse}
        <!-- tool use: secondary text + thin left rule + wrench icon -->
        <div class="trace-row" data-kind="tool">
          <Wrench size={13} class="trace-row__icon" />
          <div class="trace-row__body">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="trace-row__toggle"
                aria-expanded={expanded[r.id] ? "true" : "false"}
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.ToolResult}
        <!-- tool result: accent green + corner-down-right icon -->
        <div class="trace-row" data-kind="result">
          <CornerDownRight size={13} class="trace-row__icon" />
          <div class="trace-row__body">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="trace-row__toggle"
                aria-expanded={expanded[r.id] ? "true" : "false"}
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.SystemPrompt}
        <!-- system prompt: tertiary, collapsed by default -->
        <div class="trace-system">
          <button
            aria-expanded={expanded[r.id] ? "true" : "false"}
            onclick={() => toggleExpand(r.id)}
          >
            {#if expanded[r.id]}
              <ChevronDown size={12} />
            {:else}
              <ChevronRight size={12} />
            {/if}
            system prompt · {r.est_tokens} tokens
          </button>
          {#if expanded[r.id]}
            <pre>{r.content}</pre>
          {/if}
        </div>
      {:else}
        <div class="trace-fallback">{r.content}</div>
      {/if}
    {/each}
  </div>
{/if}

<style>
  /* ============================================================
     Conversation cards — recreation-style paper thread
     ============================================================ */
  .trace-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    margin: 0 auto;
    width: 100%;
  }

  .message {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    width: 100%;

    &[data-author="user"] {
      flex-direction: row-reverse;
    }
  }

  .message-avatar {
    width: var(--size-avatar-md);
    height: var(--size-avatar-md);
    border-radius: var(--radius-full);
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.02em;
    border: 1px solid var(--color-border-subtle);
    background: var(--color-surface-base);
    color: var(--color-text-secondary);
    overflow: hidden;
    box-shadow: 0 2px 6px rgba(17, 24, 39, 0.04);

    & img {
      width: 100%;
      height: 100%;
      display: block;
      object-fit: cover;
    }
  }

  .message[data-author="user"] .message-avatar {
    background: linear-gradient(to top, var(--blue-600), var(--blue-500));
    color: var(--white);
    border-color: var(--blue-600);
  }

  .message[data-author="agent"] .message-avatar {
    border-color: transparent;
    box-shadow: none;
  }

  .message-bubble {
    max-width: calc(100% - 44px);
    padding: var(--space-4) var(--space-5);
    border-radius: var(--radius-xl);
    font-family: var(--font-family-ui);
    font-size: 0.92rem;
    line-height: 1.6;
    color: var(--color-text-body);
    word-break: break-word;
    border: 1px solid var(--color-border-subtle);
    background: var(--color-surface-base);
    box-shadow: var(--shadow-message);

    /* `{@html ...}` content — escape Svelte's scope hash so the rules
       hit the rendered markdown nodes. */
    & :global(:first-child) { margin-top: 0; }
    & :global(:last-child) { margin-bottom: 0; }
  }

  .message[data-author="user"] .message-bubble {
    background: linear-gradient(180deg, var(--color-surface-base), var(--color-action-primary-soft));
    border-color: var(--blue-100);
  }

  /* ============================================================
     Trace rows — tool use, tool result, system prompt, fallback
     ============================================================ */
  .trace-row {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding: 10px var(--space-3);
    font-family: var(--font-family-code);
    font-size: 0.75rem;
    line-height: 1.55;
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
  }

  /* Lucide renders the icon inside its own <svg>; the class travels through
     as a prop, so target it globally to bypass Svelte's scope hash. */
  :global(.trace-row__icon) {
    margin-top: 2px;
    flex-shrink: 0;
  }

  .trace-row__body {
    flex: 1;
    word-break: break-all;
  }

  .trace-row__toggle {
    margin-left: var(--space-2);
    background: var(--color-surface-base);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-full);
    padding: 2px 8px;
    cursor: pointer;
    font-size: 0.7rem;
    color: var(--color-text-tertiary);
    font-family: inherit;
    transition: color var(--motion-duration-fast) var(--motion-ease);

    &:hover {
      color: var(--color-action-primary);
      border-color: var(--color-border-accent);
    }
  }

  .trace-row[data-kind="tool"] {
    color: var(--color-text-secondary);
    background: var(--color-surface-base);
  }

  .trace-row[data-kind="result"] {
    color: var(--gray-700);
    background: var(--gray-50);
    border-color: var(--gray-200);

    & .trace-row__toggle:hover {
      color: var(--color-action-primary);
    }
  }

  .trace-system {
    font-size: 0.7rem;
    color: var(--color-text-tertiary);
    padding: 0;
    display: block;

    & button {
      cursor: pointer;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      background: transparent;
      border: 0;
      padding: 0;
      color: inherit;
      font-family: inherit;
      font-size: inherit;
      transition:
        color var(--motion-duration-fast) var(--motion-ease),
        text-decoration-color var(--motion-duration-fast) var(--motion-ease);

      &:hover {
        color: var(--color-action-primary);
        text-decoration: underline;
        text-underline-offset: 2px;
      }
    }

    & pre {
      margin-top: var(--space-2);
      padding: var(--space-3);
      border: 1px solid var(--color-border-subtle);
      border-radius: var(--radius-lg);
      background: var(--color-surface-raised);
      white-space: pre-wrap;
      word-break: break-all;
      color: var(--color-text-secondary);
      line-height: 1.55;
      font-family: var(--font-family-code);
    }
  }

  .trace-fallback {
    font-family: var(--font-family-code);
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    padding: var(--space-3);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-lg);
    background: var(--color-surface-raised);
    word-break: break-all;
  }

  .trace-empty {
    color: var(--color-text-secondary);
    font-family: var(--font-family-display);
    font-size: 1.25rem;
    text-align: center;
    padding-top: var(--space-8);
  }
</style>
