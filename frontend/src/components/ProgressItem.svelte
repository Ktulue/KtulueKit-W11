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
    padding: var(--spacing-sm) var(--spacing-xl);
  }

  .installing {
    opacity: 0.7;
  }

  .main-line {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
  }

  .counter {
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    min-width: 60px;
    flex-shrink: 0;
  }

  .icon {
    font-size: 14px;
    min-width: 20px;
    text-align: center;
    flex-shrink: 0;
  }

  .name {
    flex: 1;
    font-size: var(--font-size-base);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .elapsed {
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    flex-shrink: 0;
  }

  .raw-toggle {
    cursor: pointer;
    color: var(--color-text-secondary);
    font-size: var(--font-size-xs);
    padding: 2px 0 2px 68px;
    transition: color 100ms ease;
  }

  .raw-toggle:hover {
    color: var(--color-text-tertiary);
  }

  .raw-output {
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: var(--spacing-md);
    margin: var(--spacing-xs) 0 var(--spacing-xs) 68px;
    font-size: var(--font-size-xs);
    white-space: pre-wrap;
    word-break: break-all;
    max-height: 200px;
    overflow-y: auto;
  }
</style>
