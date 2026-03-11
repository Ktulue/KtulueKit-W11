<script>
  export let item       // { ID, Name, Description, Notes }
  export let checked = false
  export let onChange = (id, checked) => {}

  let showTooltip = false
  $: tooltip = item.Description || item.Notes || ''
</script>

<div class="item-row">
  <label>
    <input
      type="checkbox"
      checked={checked}
      on:change={(e) => onChange(item.ID, e.target.checked)}
    />
    <span class="name">{item.Name}</span>
  </label>
  {#if tooltip}
    <span
      class="tooltip-trigger"
      on:mouseenter={() => showTooltip = true}
      on:mouseleave={() => showTooltip = false}
    >?
      {#if showTooltip}
        <span class="tooltip">{tooltip}</span>
      {/if}
    </span>
  {/if}
</div>

<style>
  .item-row {
    display: flex;
    align-items: center;
    padding: 4px 8px;
    gap: 8px;
  }
  label {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    flex: 1;
  }
  .tooltip-trigger {
    position: relative;
    cursor: help;
    color: #888;
    font-size: 12px;
    border: 1px solid #555;
    border-radius: 50%;
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .tooltip {
    position: absolute;
    right: 0;
    top: 20px;
    background: #2a2a2a;
    border: 1px solid #555;
    border-radius: 4px;
    padding: 8px;
    width: 260px;
    font-size: 12px;
    z-index: 100;
    white-space: normal;
    line-height: 1.4;
  }
</style>
