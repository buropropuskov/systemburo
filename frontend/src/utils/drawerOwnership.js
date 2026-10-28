/** Synchronous state ownership; manual/system setters revoke tour restoration. */
export function createDrawerOwnership({ read, write }) {
  let revision = 0, tokenSeen = 0, revokedThrough = 0;
  let lease = null, disposed = false;
  const validToken = token => Number.isSafeInteger(token) && token > 0;

  function set(open) {
    if (disposed) return false;
    const changed = Boolean(read()) !== Boolean(open);
    revokedThrough = Math.max(revokedThrough, tokenSeen);
    revision += 1; lease = null;
    write(Boolean(open));
    return changed;
  }

  function claim(open, token, previousToken) {
    if (disposed || !validToken(token) || token <= revokedThrough || token < tokenSeen) return false;
    const before = Boolean(read()), requested = Boolean(open);
    const previous = lease && (lease.token === token || lease.token === previousToken) ? lease : null;
    tokenSeen = token;
    if (before === requested) {
      lease = previous ? { ...previous, token, last: requested } : null;
      return false;
    }
    revision += 1;
    lease = { token, before: previous ? previous.before : before, last: requested, revision };
    write(requested);
    return true;
  }

  function restore(token) {
    if (disposed || !lease || lease.token !== token || lease.revision !== revision
      || Boolean(read()) !== lease.last) return false;
    const snapshot = lease;
    lease = null; revision += 1;
    revokedThrough = Math.max(revokedThrough, token);
    write(snapshot.before);
    return true;
  }

  function dispose() { disposed = true; lease = null; revision += 1; }
  return { set, claim, restore, dispose };
}
