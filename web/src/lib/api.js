// Fetch wrappers for the tor-dice Go server's REST endpoints.
// All endpoints are same-origin (the Go server hosts both the static
// SPA and these endpoints), so no CORS concerns. Errors come back as
// { error: "message" } with a non-2xx status.

async function postJSON(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch (e) {
    throw new Error(`bad json from ${path}: ${text.slice(0, 200)}`);
  }
  if (!res.ok) {
    throw new Error(parsed.error || `${path} returned ${res.status}`);
  }
  return parsed;
}

export const roll = (spec) => postJSON('/api/roll', { spec });

export const rollTORCheck = (args) =>
  postJSON('/api/roll_tor_check', {
    skill_rating: args.skill_rating ?? 0,
    target_number: args.target_number ?? 14,
    weariness: !!args.weariness,
    miserable: !!args.miserable,
    format: 'html_tor',
  });

export const rollTORCombat = (args) =>
  postJSON('/api/roll_tor_combat', {
    attacker_skill: args.attacker_skill ?? 0,
    defender_tn: args.defender_tn ?? 14,
    weariness: !!args.weariness,
    miserable: !!args.miserable,
    format: 'html_tor',
  });
