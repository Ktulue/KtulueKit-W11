<script>
  import { afterUpdate } from 'svelte'
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
      <ProgressItem {event} />
    {/each}
  </div>

  {#if rebootEvent}
    <div class="reboot-modal">
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
  .screen { display: flex; flex-direction: column; height: 100vh; }
  header {
    padding: 12px 20px;
    background: #111;
    border-bottom: 1px solid #333;
    display: flex;
    align-items: center;
    gap: 12px;
  }
  h2 { margin: 0; font-size: 16px; }
  .progress-bar-wrap {
    flex: 1;
    background: #333;
    border-radius: 4px;
    height: 8px;
    overflow: hidden;
  }
  .progress-bar {
    height: 100%;
    background: #0e7fd4;
    transition: width 0.3s ease;
  }
  .progress-label { color: #888; font-size: 13px; min-width: 50px; text-align: right; }
  .feed { flex: 1; overflow-y: auto; }
  .reboot-modal {
    position: absolute;
    inset: 0;
    background: rgba(0,0,0,0.8);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
  }
  .reboot-modal h3 { margin: 0; }
  .modal-buttons { display: flex; gap: 12px; }
  .reboot-btn {
    background: #c0392b;
    color: white;
    border: none;
    padding: 10px 24px;
    border-radius: 4px;
    cursor: pointer;
  }
  .cancel-btn {
    background: #2a2a2a;
    color: #e0e0e0;
    border: 1px solid #555;
    padding: 10px 24px;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
