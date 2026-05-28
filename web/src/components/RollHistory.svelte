<script>
  let { entries } = $props();

  function fmtTime(ts) {
    const d = new Date(ts);
    const hh = String(d.getHours()).padStart(2, '0');
    const mm = String(d.getMinutes()).padStart(2, '0');
    return `${hh}:${mm}`;
  }

  function summary(entry) {
    const r = entry.result;
    if (entry.kind === 'roll') {
      const mod = r.modifier ? (r.modifier > 0 ? `+${r.modifier}` : r.modifier) : '';
      return `${r.spec}: [${r.rolls.join(', ')}]${mod} = ${r.total}`;
    }
    if (entry.kind === 'check') {
      return `Skill check: total ${r.total} vs TN ${r.target_number} — ${r.succeeds ? 'succeeded' : 'failed'} by ${Math.abs(r.margin)}`;
    }
    if (entry.kind === 'combat') {
      return `Attack: total ${r.total} vs TN ${r.defender_tn} — ${r.hits ? 'hit' : 'miss'} by ${Math.abs(r.margin)}`;
    }
    return JSON.stringify(r);
  }
</script>

<div class="history">
  {#if entries.length === 0}
    <p class="history-empty">No rolls yet. Make one above.</p>
  {:else}
    {#each entries as entry (entry.ts)}
      <div class="history-entry">
        <div class="meta">{fmtTime(entry.ts)} · {entry.kind}</div>
        <div class="body">
          {#if entry.result.formatted}
            {@html entry.result.formatted}
          {:else}
            {summary(entry)}
          {/if}
        </div>
      </div>
    {/each}
  {/if}
</div>
