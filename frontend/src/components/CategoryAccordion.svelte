<script>
  import ItemRow from './ItemRow.svelte'

  export let category    // { Name, Items: [{ ID, Name, ... }] }
  export let selected    // Set<string>
  export let onToggle = (id, checked) => {}

  let open = true

  $: allChecked = category.Items.every(i => selected.has(i.ID))
  $: someChecked = category.Items.some(i => selected.has(i.ID))

  function toggleAll() {
    const shouldCheck = !allChecked
    category.Items.forEach(i => onToggle(i.ID, shouldCheck))
  }
</script>

<div class="accordion">
  <div class="header" on:click={() => open = !open}>
    <span class="arrow">{open ? '▼' : '▶'}</span>
    <span class="cat-name">{category.Name}</span>
    <span class="count">({category.Items.length})</span>
    <button class="select-all" on:click|stopPropagation={toggleAll}>
      {allChecked ? 'Deselect all' : 'Select all'}
    </button>
  </div>
  {#if open}
    <div class="items">
      {#each category.Items as item (item.ID)}
        <ItemRow
          {item}
          checked={selected.has(item.ID)}
          onChange={onToggle}
        />
      {/each}
    </div>
  {/if}
</div>

<style>
  .accordion {
    margin-bottom: var(--spacing-xs);
  }

  .header {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
    padding: var(--spacing-md) var(--spacing-lg);
    background: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    user-select: none;
    transition: background 100ms ease;
  }

  .header:hover {
    background: var(--color-bg-hover);
  }

  .arrow {
    font-size: var(--font-size-xs);
    color: var(--color-text-secondary);
    min-width: 10px;
    text-align: center;
  }

  .cat-name {
    font-weight: 600;
    flex: 1;
    font-size: var(--font-size-base);
  }

  .count {
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
  }

  .select-all {
    font-size: var(--font-size-sm);
    background: transparent;
    border: 1px solid var(--color-border-input);
    color: var(--color-text-tertiary);
    padding: var(--spacing-xs) var(--spacing-md);
    border-radius: var(--radius);
    cursor: pointer;
    transition: border-color 100ms ease, color 100ms ease;
  }

  .select-all:hover {
    border-color: var(--color-text-primary);
    color: var(--color-text-primary);
  }

  .items {
    padding-left: var(--spacing-md);
  }
</style>
