// Roll history persisted in localStorage so it survives page reload.
// Keeps the last N entries; older ones are evicted FIFO.

const KEY = 'tor-dice.history.v1';
const MAX = 50;

export function load() {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function append(entry) {
  const existing = load();
  const next = [{ ts: Date.now(), ...entry }, ...existing].slice(0, MAX);
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    // Storage full / disabled — silently drop. History is best-effort.
  }
  return next;
}

export function clear() {
  try {
    localStorage.removeItem(KEY);
  } catch {}
  return [];
}
