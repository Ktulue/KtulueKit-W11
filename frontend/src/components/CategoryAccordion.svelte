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
  .accordion { margin-bottom: 4px; }
  .header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: #2a2a2a;
    cursor: pointer;
    user-select: none;
    border-radius: 4px;
  }
  .header:hover { background: #333; }
  .arrow { font-size: 10px; color: #888; }
  .cat-name { font-weight: 600; flex: 1; }
  .count { color: #888; font-size: 13px; }
  .select-all {
    font-size: 12px;
    background: transparent;
    border: 1px solid #555;
    color: #aaa;
    padding: 2px 8px;
    border-radius: 3px;
    cursor: pointer;
  }
  .select-all:hover { border-color: #aaa; color: #fff; }
  .items { padding-left: 8px; }
</style>
