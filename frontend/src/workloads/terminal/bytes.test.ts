import {describe, expect, it} from "vitest";
import {binary, fromBase64, joined, toBase64, typed} from "./bytes";

describe("bytes", () => {
  it("carries bytes through base64 both ways", () => {
    const data = Uint8Array.from([0, 1, 127, 128, 255, 0xe2, 0x82, 0xac]);

    expect(toBase64(data)).toBe("AAF/gP/igqw=");
    expect(fromBase64("AAF/gP/igqw=")).toEqual(data);
  });

  it("carries more than one chunk", () => {
    const large = new Uint8Array(0x8000 * 2 + 3).map((_, i) => i % 256);

    expect(fromBase64(toBase64(large))).toEqual(large);
  });

  it("types text as UTF-8, and a binary string a byte a character", () => {
    expect(Array.from(typed("€"))).toEqual([0xe2, 0x82, 0xac]);
    expect(binary("\x1b[M\xff")).toEqual(Uint8Array.from([0x1b, 0x5b, 0x4d, 0xff]));
  });

  it("joins bytes one after another", () => {
    expect(joined([Uint8Array.from([1]), Uint8Array.from([2, 3])])).toEqual(Uint8Array.from([1, 2, 3]));

    const one = Uint8Array.from([9]);
    expect(joined([one])).toBe(one);
  });
});
