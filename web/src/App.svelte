<script lang="ts">
  import SessionList from "./components/SessionList.svelte";
  import SessionDetail from "./components/SessionDetail.svelte";
  import { refreshSessions, selectedSession } from "./lib/stores/sessions";

  $effect(() => {
    refreshSessions();
  });
</script>

<div class="bg"></div>

<div class="min-h-screen flex flex-col">
  <main class="flex-1 min-h-0 flex">
    <div class="container">
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
