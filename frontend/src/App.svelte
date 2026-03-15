<script>
  import { onMount } from 'svelte'
  import { GetConfig, StartInstall, StartUninstall, GetInstalledItems } from '../wailsjs/go/main/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import SelectionScreen from './screens/SelectionScreen.svelte'
  import ProgressScreen from './screens/ProgressScreen.svelte'
  import SummaryScreen from './screens/SummaryScreen.svelte'

  // screen: 'selection' | 'progress' | 'summary'
  let screen = 'selection'
  let configView = null
  let progressEvents = []
  let summaryResult = null
  let adminError = null

  onMount(async () => {
    configView = await GetConfig()

    EventsOn('progress', (event) => {
      progressEvents = [...progressEvents, event]
    })

    EventsOn('complete', (result) => {
      summaryResult = result
      screen = 'summary'
    })

    EventsOn('uninstall_complete', (result) => {
      summaryResult = result
      screen = 'summary'
    })
  })

  async function handleStartInstall(selectedIds) {
    progressEvents = []
    const err = await StartInstall(selectedIds)
    if (err) {
      alert(err)
      return
    }
    screen = 'progress'
  }

  async function handleStartUninstall(selectedIds) {
    progressEvents = []
    const err = await StartUninstall(selectedIds)
    if (err) {
      alert(err)
      return
    }
    screen = 'progress'
  }

  async function handleGetInstalledItems(ids) {
    return await GetInstalledItems(ids)
  }

  function handleClose() {
    window.close()
  }
</script>

<main>
  {#if adminError}
    <div class="error-screen">
      <h1>Administrator required</h1>
      <p>Right-click the .exe and choose "Run as administrator".</p>
    </div>
  {:else if screen === 'selection'}
    <SelectionScreen
      {configView}
      onStart={handleStartInstall}
      onUninstall={handleStartUninstall}
      getInstalledItems={handleGetInstalledItems}
    />
  {:else if screen === 'progress'}
    <ProgressScreen events={progressEvents} />
  {:else if screen === 'summary'}
    <SummaryScreen result={summaryResult} onClose={handleClose} />
  {/if}
</main>

<style>
  @import url('https://fonts.googleapis.com/css2?family=Nunito:wght@400;600;700&display=swap');

  :root {
    /* Colors */
    --color-bg-primary: #1a1a1a;
    --color-bg-secondary: #111;
    --color-bg-hover: #2a2a2a;
    --color-border: #333;
    --color-border-input: #555;
    --color-text-primary: #e0e0e0;
    --color-text-secondary: #888;
    --color-text-tertiary: #aaa;
    --color-accent: #0e7fd4;
    --color-accent-hover: #1290e8;
    --color-accent-disabled: #444;
    --color-danger: #ff6b6b;
    --color-danger-action: #c0392b;

    /* Spacing (4px grid) */
    --spacing-xs: 4px;
    --spacing-sm: 6px;
    --spacing-md: 8px;
    --spacing-lg: 12px;
    --spacing-xl: 16px;
    --spacing-2xl: 20px;

    /* Shape */
    --radius: 4px;

    /* Typography */
    --font-primary: "Nunito", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    --font-size-xs: 11px;
    --font-size-sm: 12px;
    --font-size-base: 15px;
    --font-size-lg: 16px;
    --font-size-xl: 18px;
  }

  :global(body) {
    margin: 0;
    font-family: var(--font-primary);
    background: var(--color-bg-primary);
    color: var(--color-text-primary);
  }

  main {
    height: 100vh;
    overflow: hidden;
  }

  .error-screen {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100vh;
    gap: 1rem;
    color: var(--color-danger);
  }
</style>
