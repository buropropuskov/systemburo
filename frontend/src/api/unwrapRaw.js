/** Preserve the envelope and HTTP outcome for callers distinguishing access from loading errors. */
export async function unwrapRaw(response, fallback) {
  const body = await response.json();
  if (!response.ok || !body || !body.success) {
    const error = new Error(body?.error || fallback);
    error.status = response.status;
    throw error;
  }
  return body;
}
