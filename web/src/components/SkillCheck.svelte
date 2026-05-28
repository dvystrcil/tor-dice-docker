<script>
  import { rollTORCheck } from '../lib/api.js';

  let { onresult, onerror, busy = $bindable() } = $props();

  // Inputs default to a typical seasoned-hero check: Skill 2 vs TN 14.
  let skill_rating = $state(2);
  let target_number = $state(14);
  let weariness = $state(false);
  let miserable = $state(false);

  async function roll() {
    busy = true;
    try {
      const r = await rollTORCheck({
        skill_rating: Number(skill_rating),
        target_number: Number(target_number),
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
    <label for="skill">Skill Rating</label>
    <input id="skill" type="number" min="0" max="9" bind:value={skill_rating} />
  </div>
  <div>
    <label for="tn">Target Number</label>
    <input id="tn" type="number" min="0" max="30" bind:value={target_number} />
  </div>
</div>

<div class="checks">
  <label>
    <input type="checkbox" bind:checked={weariness} />
    Weary (1-3 on Success Dice → 0)
  </label>
  <label>
    <input type="checkbox" bind:checked={miserable} />
    Miserable (Eye → Shadow)
  </label>
</div>

<button class="roll" onclick={roll} disabled={busy}>
  {busy ? 'Rolling…' : 'Roll Feat + Success Dice'}
</button>
