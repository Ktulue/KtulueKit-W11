<script>
  import { onMount } from 'svelte'
  import { GetConfig, StartInstall } from '../wailsjs/go/main/App'
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
    <SelectionScreen {configView} onStart={handleStartInstall} />
  {:else if screen === 'progress'}
    <ProgressScreen events={progressEvents} />
  {:else if screen === 'summary'}
    <SummaryScreen result={summaryResult} onClose={handleClose} />
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: 'Segoe UI', sans-serif;
    background: #1a1a1a;
    color: #e0e0e0;
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
    color: #ff6b6b;
  }
</style>
