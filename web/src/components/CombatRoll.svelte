<script>
  import { rollTORCombat } from '../lib/api.js';

  let { onresult, onerror, busy = $bindable() } = $props();

  let attacker_skill = $state(2);
  let defender_tn = $state(14);
  let weariness = $state(false);
  let miserable = $state(false);

  async function roll() {
    busy = true;
    try {
      const r = await rollTORCombat({
        attacker_skill: Number(attacker_skill),
        defender_tn: Number(defender_tn),
        weariness,
        miserable,
      });
      onresult(r);
    } catch (e) {
      onerror(e.message || String(e));
    }
  }
</script>

<div class="row">
  <div>
    <label for="atk">Attacker Skill</label>
    <input id="atk" type="number" min="0" max="9" bind:value={attacker_skill} />
  </div>
  <div>
    <label for="dtn">Defender Parry TN</label>
    <input id="dtn" type="number" min="0" max="30" bind:value={defender_tn} />
  </div>
</div>

<div class="checks">
  <label>
    <input type="checkbox" bind:checked={weariness} />
    Weary
  </label>
  <label>
    <input type="checkbox" bind:checked={miserable} />
    Miserable
  </label>
</div>

<button class="roll" onclick={roll} disabled={busy}>
  {busy ? 'Rolling…' : 'Roll Attack'}
</button>
