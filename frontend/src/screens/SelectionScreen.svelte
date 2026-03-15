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
    padding: 12px 20px;
    background: #111;
    border-bottom: 1px solid #333;
    gap: 16px;
  }
  h1 { margin: 0; font-size: 18px; }
  .profile-bar { display: flex; align-items: center; gap: 8px; }
  select {
    background: #2a2a2a;
    color: #e0e0e0;
    border: 1px solid #555;
    padding: 4px 8px;
    border-radius: 4px;
  }
  .tab-bar {
    display: flex;
    border-bottom: 1px solid #333;
    background: #111;
  }
  .tab {
    padding: 8px 20px;
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    cursor: pointer;
    color: #888;
    font-size: 15px;
    transition: color 100ms ease;
  }
  .tab.tab-active { color: #0e7fd4; border-bottom-color: #0e7fd4; }
  .tab.tab-active.tab-uninstall-active { color: #ff6b6b; border-bottom-color: #ff6b6b; }
  .categories {
    flex: 1;
    overflow-y: auto;
    padding: 12px 20px;
  }
  .scan-state { padding: 32px; text-align: center; color: #888; }
  footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 20px;
    background: #111;
    border-top: 1px solid #333;
  }
  .count { color: #888; }
  .start-btn {
    background: #0e7fd4;
    color: white;
    border: none;
    padding: 10px 28px;
    border-radius: 4px;
    font-size: 15px;
    cursor: pointer;
  }
  .start-btn:disabled { background: #444; cursor: not-allowed; }
  .start-btn:not(:disabled):hover { background: #1290e8; }
  .uninstall-btn { background: #c0392b; }
  .uninstall-btn:not(:disabled):hover { background: #e74c3c; }
</style>
