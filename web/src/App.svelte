<script>
  import SkillCheck from './components/SkillCheck.svelte';
  import CombatRoll from './components/CombatRoll.svelte';
  import GenericRoll from './components/GenericRoll.svelte';
  import RollHistory from './components/RollHistory.svelte';
  import { load, append, clear } from './lib/history.js';

  let activeTab = $state('check'); // 'check' | 'combat' | 'roll'
  let history = $state(load());
  let lastResult = $state(null);
  let lastKind = $state(null);
  let error = $state(null);
  let busy = $state(false);

  // Each form component fires (result, kind) — we record into history
  // and surface the latest as `lastResult` for the inline result panel.
  function onRoll(kind, result) {
    error = null;
    busy = false;
    lastKind = kind;
    lastResult = result;
    history = append({ kind, result });
  }

  function onError(msg) {
    error = msg;
    busy = false;
  }

  function clearHistory() {
    history = clear();
  }
</script>

<h1>The One Ring</h1>
<p class="subtitle">Dice for the Lone-lands</p>

<div class="card">
  <div class="tabs" role="tablist">
    <button class="tab" class:active={activeTab === 'check'}
            onclick={() => (activeTab = 'check')}>Skill Check</button>
    <button class="tab" class:active={activeTab === 'combat'}
            onclick={() => (activeTab = 'combat')}>Combat</button>
    <button class="tab" class:active={activeTab === 'roll'}
            onclick={() => (activeTab = 'roll')}>Roll Any</button>
  </div>

  {#if activeTab === 'check'}
    <SkillCheck onresult={(r) => onRoll('check', r)} onerror={onError} bind:busy />
  {:else if activeTab === 'combat'}
    <CombatRoll onresult={(r) => onRoll('combat', r)} onerror={onError} bind:busy />
  {:else}
    <GenericRoll onresult={(r) => onRoll('roll', r)} onerror={onError} bind:busy />
  {/if}

  {#if error}
    <div class="error">{error}</div>
  {/if}

  {#if lastResult && !error}
    {@const failed = lastResult.succeeds === false || lastResult.hits === false}
    <div class="result" class:success={!failed} class:failure={failed}>
      {#if lastResult.formatted}
        {@html lastResult.formatted}
      {:else if lastKind === 'roll'}
        Rolled <strong>{lastResult.spec}</strong> → [{lastResult.rolls.join(', ')}]
        {#if lastResult.modifier}{lastResult.modifier > 0 ? '+' : ''}{lastResult.modifier}{/if}
        = <strong>{lastResult.total}</strong>
      {:else}
        Total: <strong>{lastResult.total}</strong>
      {/if}
    </div>
  {/if}
</div>

<div class="card">
  <h2>
    Recent Rolls
    {#if history.length > 0}
      <button class="history-clear" onclick={clearHistory}>Clear</button>
    {/if}
  </h2>
  <RollHistory entries={history} />
</div>
