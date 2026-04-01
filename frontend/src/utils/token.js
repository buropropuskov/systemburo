export function decodeToken(token) {
  try {
    return JSON.parse(atob(token.split('.')[1]));
  } catch {
    return null;
  }
}

export function isTokenValid(token) {
  if (!token) return false;
  const payload = decodeToken(token);
  return !!(payload && payload.exp > Math.floor(Date.now() / 1000));
}

export function getUserType(token) {
  const payload = decodeToken(token);
  return payload?.type_id || null;
}

export function getTokenExpiry(token) {
  const payload = decodeToken(token);
  return payload?.exp || 0;
}
