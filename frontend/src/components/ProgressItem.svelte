<script>
  export let event   // ProgressEvent: { Index, Total, ID, Name, Status, Detail, Elapsed }

  let rawOpen = false

  const statusIcons = {
    installing: '⏳',
    installed:  '✅',
    upgraded:   '⬆️',
    already:    '⏭️',
    failed:     '❌',
    skipped:    '⚠️',
    reboot:     '🔄',
    reboot_cancelled: '↩️',
    shortcut_removed: '🗑️',
  }
  $: icon = statusIcons[event.Status] || '•'
  $: isInstalling = event.Status === 'installing'
</script>

<div class="progress-item" class:installing={isInstalling}>
  <div class="main-line">
    <span class="counter">[{event.Index}/{event.Total}]</span>
    <span class="icon">{icon}</span>
    <span class="name">{event.Name}</span>
    {#if event.Elapsed}
      <span class="elapsed">{event.Elapsed}</span>
    {/if}
  </div>
  {#if event.Detail}
    <div class="raw-toggle" on:click={() => rawOpen = !rawOpen}>
      {rawOpen ? '▼' : '▶'} Raw Output
    </div>
    {#if rawOpen}
      <pre class="raw-output">{event.Detail}</pre>
    {/if}
  {/if}
</div>

<style>
  .progress-item {
    padding: 6px 12px;
    border-bottom: 1px solid #222;
  }
  .installing { opacity: 0.7; }
  .main-line { display: flex; align-items: center; gap: 8px; }
  .counter { color: #666; font-size: 12px; min-width: 60px; }
  .icon { font-size: 14px; }
  .name { flex: 1; }
  .elapsed { color: #888; font-size: 12px; }
  .raw-toggle {
    cursor: pointer;
    color: #666;
    font-size: 11px;
    padding: 2px 0 2px 68px;
  }
  .raw-toggle:hover { color: #aaa; }
  .raw-output {
    background: #111;
    border: 1px solid #333;
    padding: 8px;
    margin: 4px 0 4px 68px;
    font-size: 11px;
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 200px;
    overflow-y: auto;
  }
</style>
