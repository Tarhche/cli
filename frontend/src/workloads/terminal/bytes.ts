const CHUNK = 0x8000;
const encoder = new TextEncoder();

/** The bytes base64 carries, as Go's []byte is written in JSON. */
export function fromBase64(base64: string): Uint8Array {
  const binary = atob(base64);
  const decoded = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    decoded[i] = binary.charCodeAt(i);
  }

  return decoded;
}

/** Bytes as base64, which Go reads into a []byte. */
export function toBase64(data: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < data.length; i += CHUNK) {
    binary += String.fromCharCode(...data.subarray(i, i + CHUNK));
  }

  return btoa(binary);
}

/** What was typed, as the UTF-8 a terminal reads. */
export function typed(text: string): Uint8Array {
  return encoder.encode(text);
}

/** A binary string, one byte a character, as xterm reports mouse moves in. */
export function binary(text: string): Uint8Array {
  return Uint8Array.from(text, (character) => character.charCodeAt(0) & 0xff);
}

/** Bytes one after another. */
export function joined(chunks: Uint8Array[]): Uint8Array {
  if (chunks.length === 1) {
    return chunks[0];
  }

  const total = chunks.reduce((length, chunk) => length + chunk.length, 0);
  const all = new Uint8Array(total);

  let at = 0;
  for (const chunk of chunks) {
    all.set(chunk, at);
    at += chunk.length;
  }

  return all;
}
