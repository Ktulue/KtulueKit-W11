<script>
  import { fade } from 'svelte/transition'

  export let result  // SummaryResult
  export let onClose = () => {}

  const sections = [
    { key: 'Installed',        label: 'Installed successfully',        color: 'accent'     },
    { key: 'Upgraded',         label: 'Updated to newer version',      color: 'accent'     },
    { key: 'Already',          label: 'Already installed (skipped)',   color: 'secondary'  },
    { key: 'Failed',           label: 'Failed',                        color: 'danger'     },
    { key: 'Skipped',          label: 'Skipped (dependency missing)',  color: 'secondary'  },
    { key: 'Reboot',           label: 'Reboot required',               color: 'accent'     },
    { key: 'ShortcutsRemoved', label: 'Desktop shortcuts removed',     color: 'secondary'  },
  ]

  function copyLogPath() {
    navigator.clipboard.writeText(result.LogPath)
  }
</script>

<div class="screen">
  <header>
    <h2>Summary</h2>
    <span class="elapsed">Total elapsed: {result.TotalElapsed}</span>
  </header>

  <div class="sections">
    {#each sections as s, i}
      {#if result[s.key] && result[s.key].length > 0}
        <div
          class="section"
          in:fade={{ duration: 150, delay: i * 50 }}
        >
          <h3 class="section-header section-{s.color}">
            {s.label}
            <span class="badge badge-{s.color}">{result[s.key].length}</span>
          </h3>
          <ul>
            {#each result[s.key] as name}
              <li>{name}</li>
            {/each}
          </ul>
        </div>
      {/if}
    {/each}
  </div>

  <footer>
    <div class="log-path">
      <span class="log-label">Log:</span>
      <span class="log-value">{result.LogPath}</span>
      <button class="copy-btn" on:click={copyLogPath}>Copy</button>
    </div>
    <button class="close-btn" on:click={onClose}>Close</button>
  </footer>
</div>

<style>
  .screen {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    display: flex;
    align-items: center;
    padding: var(--spacing-lg) var(--spacing-2xl);
    background: var(--color-bg-secondary);
    border-bottom: 1px solid var(--color-border);
    gap: var(--spacing-xl);
  }

  h2 {
    margin: 0;
    font-size: var(--font-size-lg);
  }

  .elapsed {
    color: var(--color-text-secondary);
  }

  .sections {
    flex: 1;
    overflow-y: auto;
    padding: var(--spacing-xl) var(--spacing-2xl);
  }

  .section {
    margin-bottom: var(--spacing-2xl);
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
    margin: 0 0 var(--spacing-md);
    font-size: var(--font-size-lg);
  }

  .section-accent  { color: var(--color-accent); }
  .section-danger  { color: var(--color-danger); }
  .section-secondary { color: var(--color-text-secondary); }

  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: var(--spacing-xs) var(--spacing-md);
    border-radius: 999px;
    font-size: var(--font-size-sm);
    font-weight: 600;
    line-height: 1;
  }

  .badge-accent {
    background: rgba(14, 127, 212, 0.15);
    color: var(--color-accent);
    border: 1px solid rgba(14, 127, 212, 0.35);
  }

  .badge-danger {
    background: rgba(255, 107, 107, 0.15);
    color: var(--color-danger);
    border: 1px solid rgba(255, 107, 107, 0.35);
  }

  .badge-secondary {
    background: var(--color-bg-hover);
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }

  ul {
    margin: 0;
    padding-left: var(--spacing-2xl);
  }

  li {
    padding: var(--spacing-xs) 0;
    color: var(--color-text-primary);
    font-size: var(--font-size-base);
  }

  footer {
    padding: var(--spacing-lg) var(--spacing-2xl);
    background: var(--color-bg-secondary);
    border-top: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .log-path {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
    font-size: var(--font-size-sm);
  }

  .log-label {
    color: var(--color-text-secondary);
  }

  .log-value {
    color: var(--color-text-tertiary);
    max-width: 400px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .copy-btn {
    background: transparent;
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
    padding: var(--spacing-xs) var(--spacing-md);
    border-radius: var(--radius);
    font-size: var(--font-size-sm);
    font-family: var(--font-primary);
    cursor: pointer;
    transition: border-color 100ms ease, color 100ms ease;
  }

  .copy-btn:hover {
    border-color: var(--color-text-tertiary);
    color: var(--color-text-primary);
  }

  .close-btn {
    background: var(--color-accent);
    color: var(--color-text-primary);
    border: none;
    padding: 10px 28px;
    border-radius: var(--radius);
    font-size: var(--font-size-base);
    font-family: var(--font-primary);
    cursor: pointer;
    transition: background 100ms ease, transform 80ms ease;
  }

  .close-btn:hover {
    background: var(--color-accent-hover);
  }

  .close-btn:active {
    transform: scale(0.98);
  }
</style>
