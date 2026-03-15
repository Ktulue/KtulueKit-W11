<script>
  import { afterUpdate } from 'svelte'
  import { fade } from 'svelte/transition'
  import ProgressItem from '../components/ProgressItem.svelte'
  import { ConfirmReboot, CancelReboot } from '../../wailsjs/go/main/App'

  export let events = []

  let feedEl

  $: rebootEvent = events.find(e => e.Status === 'reboot' && !events.find(f => f.Status === 'reboot_cancelled'))
  $: latestIndex = events.filter(e => e.Index > 0).reduce((max, e) => Math.max(max, e.Index), 0)
  $: total = events.length > 0 ? events[events.length - 1].Total || 0 : 0
  $: progress = total > 0 ? Math.round((latestIndex / total) * 100) : 0

  afterUpdate(() => {
    if (feedEl) feedEl.scrollTop = feedEl.scrollHeight
  })
</script>

<div class="screen">
  <header>
    <h2>Installing...</h2>
    <div class="progress-bar-wrap">
      <div class="progress-bar" style="width: {progress}%"></div>
    </div>
    <span class="progress-label">{latestIndex}/{total}</span>
  </header>

  <div class="feed" bind:this={feedEl}>
    {#each events as event, i (i)}
      <div in:fade={{ duration: 150 }}>
        <ProgressItem {event} />
      </div>
    {/each}
  </div>

  {#if rebootEvent}
    <div class="reboot-modal" in:fade={{ duration: 100 }}>
      <h3>{rebootEvent.Name} requires a reboot</h3>
      <p>The auto-resume task has been registered and will run after login.</p>
      <div class="modal-buttons">
        <button class="reboot-btn" on:click={ConfirmReboot}>Reboot Now</button>
        <button class="cancel-btn" on:click={CancelReboot}>Continue Without Rebooting</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .screen {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    padding: var(--spacing-lg) var(--spacing-2xl);
    background: var(--color-bg-secondary);
    border-bottom: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    gap: var(--spacing-lg);
  }

  h2 {
    margin: 0;
    font-size: var(--font-size-lg);
  }

  .progress-bar-wrap {
    flex: 1;
    background: var(--color-border);
    border-radius: var(--radius);
    height: var(--spacing-md);
    overflow: hidden;
  }

  .progress-bar {
    height: 100%;
    background: var(--color-accent);
    transition: width 0.3s ease;
  }

  .progress-label {
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    min-width: 50px;
    text-align: right;
  }

  .feed {
    flex: 1;
    overflow-y: auto;
    background: var(--color-bg-secondary);
  }

  .reboot-modal {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--spacing-xl);
  }

  .reboot-modal h3 {
    margin: 0;
  }

  .modal-buttons {
    display: flex;
    gap: var(--spacing-lg);
  }

  .reboot-btn {
    background: var(--color-accent);
    color: var(--color-text-primary);
    border: none;
    padding: 10px 24px;
    border-radius: var(--radius);
    font-family: var(--font-primary);
    cursor: pointer;
    transition: background 100ms ease;
  }

  .reboot-btn:hover {
    background: var(--color-accent-hover);
  }

  .cancel-btn {
    background: var(--color-bg-hover);
    color: var(--color-text-primary);
    border: 1px solid var(--color-border-input);
    padding: 10px 24px;
    border-radius: var(--radius);
    font-family: var(--font-primary);
    cursor: pointer;
    transition: background 100ms ease, border-color 100ms ease;
  }

  .cancel-btn:hover {
    background: var(--color-border);
    border-color: var(--color-text-tertiary);
  }
</style>
