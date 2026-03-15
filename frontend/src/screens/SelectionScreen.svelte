<script>
  import CategoryAccordion from '../components/CategoryAccordion.svelte'

  export let configView  // { Categories, Profiles }
  export let onStart = (ids) => {}
  export let onUninstall = (ids) => {}
  export let getInstalledItems = async (ids) => []

  let selected = new Set()
  let profileName = ''

  // Uninstall tab state
  let activeTab = 'install'
  let scanLoading = false
  let installedIDs = []
  let scanDone = false
  let uninstallSelected = new Set()

  $: allItems = configView
    ? configView.Categories.flatMap(c => c.Items)
    : []

  $: hasUninstallSelection = uninstallSelected.size > 0

  function getAllItemIDs() {
    return allItems.map(i => i.ID)
  }

  async function switchTab(tab) {
    activeTab = tab
    if (tab === 'uninstall' && !scanDone) {
      scanLoading = true
      installedIDs = await getInstalledItems(getAllItemIDs())
      scanLoading = false
      scanDone = true
    }
  }

  // Build filtered categories for uninstall tab (only installed items)
  $: uninstallCategories = configView
    ? configView.Categories
        .map(cat => ({
          ...cat,
          Items: cat.Items.filter(i => installedIDs.includes(i.ID))
        }))
        .filter(cat => cat.Items.length > 0)
    : []

  function loadProfile(name) {
    profileName = name
    if (!name) return
    const profile = configView.Profiles.find(p => p.Name === name)
    if (!profile) return
    selected = new Set(profile.IDs)
  }

  function toggleItem(id, checked) {
    selected = new Set(selected)
    if (checked) selected.add(id)
    else selected.delete(id)
    profileName = '' // clear profile label when user adjusts manually
  }

  function toggleUninstallItem(id, checked) {
    uninstallSelected = new Set(uninstallSelected)
    if (checked) uninstallSelected.add(id)
    else uninstallSelected.delete(id)
  }

  function handleStart() {
    onStart([...selected])
  }

  function handleUninstall() {
    onUninstall([...uninstallSelected])
  }
</script>

<div class="screen">
  <header>
    <h1>KtulueKit</h1>
    {#if activeTab === 'install'}
      <div class="profile-bar">
        <label>Profile:</label>
        <select bind:value={profileName} on:change={(e) => loadProfile(e.target.value)}>
          <option value="">— custom —</option>
          {#if configView}
            {#each configView.Profiles as profile}
              <option value={profile.Name}>{profile.Name}</option>
            {/each}
          {/if}
        </select>
      </div>
    {/if}
  </header>

  <div class="tab-bar">
    <button
      class="tab"
      class:tab-active={activeTab === 'install'}
      on:click={() => switchTab('install')}
    >Install</button>
    <button
      class="tab"
      class:tab-active={activeTab === 'uninstall'}
      class:tab-uninstall-active={activeTab === 'uninstall'}
      on:click={() => switchTab('uninstall')}
    >Uninstall</button>
  </div>

  <div class="categories">
    {#if activeTab === 'install'}
      {#if configView}
        {#each configView.Categories as category (category.Name)}
          <CategoryAccordion
            {category}
            selected={selected}
            onToggle={toggleItem}
          />
        {/each}
      {:else}
        <p>Loading config...</p>
      {/if}
    {:else}
      {#if scanLoading}
        <div class="scan-state">Scanning installed items...</div>
      {:else if !scanDone || installedIDs.length === 0}
        {#if scanDone}
          <div class="scan-state">No installed items detected.</div>
        {:else}
          <div class="scan-state">Switch to this tab to scan for installed items.</div>
        {/if}
      {:else}
        {#each uninstallCategories as category (category.Name)}
          <CategoryAccordion
            {category}
            selected={uninstallSelected}
            onToggle={toggleUninstallItem}
          />
        {/each}
      {/if}
    {/if}
  </div>

  <footer>
    {#if activeTab === 'install'}
      <span class="count">{selected.size} item{selected.size === 1 ? '' : 's'} selected</span>
      <button
        class="start-btn"
        disabled={selected.size === 0}
        on:click={handleStart}
      >
        Start Install
      </button>
    {:else}
      <span class="count">{uninstallSelected.size} item{uninstallSelected.size === 1 ? '' : 's'} selected</span>
      <button
        class="start-btn uninstall-btn"
        disabled={!hasUninstallSelection}
        on:click={handleUninstall}
      >
        Uninstall Selected
      </button>
    {/if}
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

  h1 {
    margin: 0;
    font-size: var(--font-size-xl);
  }

  .profile-bar {
    display: flex;
    align-items: center;
    gap: var(--spacing-md);
  }

  select {
    background: var(--color-bg-hover);
    color: var(--color-text-primary);
    border: 1px solid var(--color-border-input);
    padding: var(--spacing-xs) var(--spacing-md);
    border-radius: var(--radius);
    transition: border-color 100ms ease;
  }

  select:hover {
    border-color: var(--color-text-tertiary);
  }

  .tab-bar {
    display: flex;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-secondary);
  }

  .tab {
    padding: var(--spacing-md) var(--spacing-2xl);
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    cursor: pointer;
    color: var(--color-text-secondary);
    font-size: var(--font-size-base);
    font-family: var(--font-primary);
    transition: color 100ms ease-out, border-bottom-color 100ms ease-out;
  }

  .tab.tab-active {
    color: var(--color-accent);
    border-bottom-color: var(--color-accent);
  }

  .tab.tab-active.tab-uninstall-active {
    color: var(--color-danger);
    border-bottom-color: var(--color-danger);
  }

  .categories {
    flex: 1;
    overflow-y: auto;
    padding: var(--spacing-lg) var(--spacing-2xl);
  }

  .scan-state {
    padding: 32px;
    text-align: center;
    color: var(--color-text-secondary);
  }

  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--spacing-lg) var(--spacing-2xl);
    background: var(--color-bg-secondary);
    border-top: 1px solid var(--color-border);
  }

  .count {
    color: var(--color-text-secondary);
  }

  .start-btn {
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

  .start-btn:disabled {
    background: var(--color-accent-disabled);
    cursor: not-allowed;
  }

  .start-btn:not(:disabled):hover {
    background: var(--color-accent-hover);
  }

  .start-btn:not(:disabled):active {
    transform: scale(0.98);
  }

  .uninstall-btn {
    background: var(--color-danger-action);
  }

  .uninstall-btn:not(:disabled):hover {
    background: #e74c3c;
  }
</style>
