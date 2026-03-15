<script>
  export let item       // { ID, Name, Description, Notes }
  export let checked = false
  export let onChange = (id, checked) => {}
  export let mode = 'install' // 'install' | 'uninstall'

  let showTooltip = false
  $: tooltip = item.Description || item.Notes || ''
</script>

<div class="item-row" class:uninstall-mode={mode === 'uninstall'}>
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
    --item-accent: var(--color-accent);
    display: flex;
    align-items: center;
    padding: var(--spacing-xs) var(--spacing-xl);
    gap: var(--spacing-md);
    border-radius: var(--radius);
    transition: background 100ms ease;
  }

  .item-row:hover {
    background: var(--color-bg-hover);
  }

  .item-row.uninstall-mode {
    --item-accent: var(--color-danger);
  }

  label {
    display: flex;
    align-items: center;
    gap: var(--spacing-sm);
    cursor: pointer;
    flex: 1;
    font-size: var(--font-size-base);
  }

  input[type='checkbox'] {
    cursor: pointer;
    accent-color: var(--item-accent);
  }

  .name {
    color: var(--color-text-primary);
  }

  .tooltip-trigger {
    position: relative;
    cursor: help;
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    border: 1px solid var(--color-border-input);
    border-radius: 50%;
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .tooltip {
    position: absolute;
    right: 0;
    top: 20px;
    background: var(--color-bg-hover);
    border: 1px solid var(--color-border-input);
    border-radius: var(--radius);
    padding: var(--spacing-md);
    width: 260px;
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    z-index: 100;
    white-space: normal;
    line-height: 1.4;
  }
</style>
