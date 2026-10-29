/** Parse only literal test-id alternatives; a malformed branch invalidates the whole selector. */
export function tourTestIds(selector) {
  if (selector === null) return [];
  if (typeof selector !== 'string' || !selector.trim()) return null;
  const matches = selector.split(',').map(part => part.trim().match(/^\[data-testid="([^"]+)"\]$/));
  return matches.every(Boolean) ? matches.map(match => match[1]) : null;
}
