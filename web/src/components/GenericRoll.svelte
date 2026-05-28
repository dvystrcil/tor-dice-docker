<script>
  import { roll as apiRoll } from '../lib/api.js';

  let { onresult, onerror, busy = $bindable() } = $props();

  let spec = $state('2d6+1');

  async function roll() {
    busy = true;
    try {
      const r = await apiRoll(spec);
      onresult(r);
    } catch (e) {
      onerror(e.message || String(e));
    }
  }
</script>

<label for="spec">Dice expression</label>
<input id="spec" type="text" bind:value={spec} placeholder="e.g. 2d6+1, 1d20, d12, 3d8-2" />

<button class="roll" onclick={roll} disabled={busy || !spec.trim()}>
  {busy ? 'Rolling…' : `Roll ${spec || '…'}`}
</button>
