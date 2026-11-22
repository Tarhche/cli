/**
 * A new id, random enough that two terminals never share one. A page that
 * is not a secure context, as a webview's own scheme may not be, has no
 * crypto.randomUUID, but every page has crypto.getRandomValues.
 */
export function newID(random: Pick<Crypto, "getRandomValues"> & Partial<Pick<Crypto, "randomUUID">> = crypto): string {
  if (typeof random.randomUUID === "function") {
    return random.randomUUID();
  }

  const bytes = random.getRandomValues(new Uint8Array(16));

  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}
