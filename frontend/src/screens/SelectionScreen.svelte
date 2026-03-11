<script>
  import CategoryAccordion from '../components/CategoryAccordion.svelte'

  export let configView  // { Categories, Profiles }
  export let onStart = (ids) => {}

  let selected = new Set()
  let profileName = ''

  $: allItems = configView
    ? configView.Categories.flatMap(c => c.Items)
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

  function handleStart() {
    onStart([...selected])
  }
</script>

<div class="screen">
  <header>
    <h1>KtulueKit</h1>
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
  </header>

  <div class="categories">
    {#if configView}
      {#each configView.Categories as category (category.Name)}
        <CategoryAccordion
          {category}
          {selected}
          onToggle={toggleItem}
        />
      {/each}
    {:else}
      <p>Loading config...</p>
    {/if}
  </div>

  <footer>
    <span class="count">{selected.size} item{selected.size === 1 ? '' : 's'} selected</span>
    <button
      class="start-btn"
      disabled={selected.size === 0}
      on:click={handleStart}
    >
      Start Install
    </button>
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
  .categories {
    flex: 1;
    overflow-y: auto;
    padding: 12px 20px;
  }
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
</style>
