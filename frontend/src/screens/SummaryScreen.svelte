<script>
  export let result  // SummaryResult
  export let onClose = () => {}

  const sections = [
    { key: 'Installed',        label: 'Installed successfully' },
    { key: 'Upgraded',         label: 'Updated to newer version' },
    { key: 'Already',          label: 'Already installed (skipped)' },
    { key: 'Failed',           label: 'Failed' },
    { key: 'Skipped',          label: 'Skipped (dependency missing)' },
    { key: 'Reboot',           label: 'Reboot required' },
    { key: 'ShortcutsRemoved', label: 'Desktop shortcuts removed' },
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
    {#each sections as s}
      {#if result[s.key] && result[s.key].length > 0}
        <div class="section">
          <h3>{s.label} ({result[s.key].length})</h3>
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
      <button on:click={copyLogPath}>Copy</button>
    </div>
    <button class="close-btn" on:click={onClose}>Close</button>
  </footer>
</div>

<style>
  .screen { display: flex; flex-direction: column; height: 100vh; }
  header {
    display: flex;
    align-items: center;
    padding: 12px 20px;
    background: #111;
    border-bottom: 1px solid #333;
    gap: 16px;
  }
  h2 { margin: 0; }
  .elapsed { color: #888; }
  .sections { flex: 1; overflow-y: auto; padding: 16px 20px; }
  .section { margin-bottom: 20px; }
  h3 { margin: 0 0 8px; font-size: 14px; color: #aaa; }
  ul { margin: 0; padding-left: 20px; }
  li { padding: 2px 0; }
  footer {
    padding: 12px 20px;
    background: #111;
    border-top: 1px solid #333;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .log-path { display: flex; align-items: center; gap: 8px; font-size: 12px; }
  .log-label { color: #888; }
  .log-value { color: #aaa; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .close-btn {
    background: #0e7fd4;
    color: white;
    border: none;
    padding: 10px 28px;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
