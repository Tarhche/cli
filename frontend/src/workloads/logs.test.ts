import {describe, expect, it} from "vitest";
import {lineOf} from "./logs";

describe("lineOf", () => {
  it("says when, from where and what", () => {
    const at = "2026-11-22T10:00:00Z";
    const when = new Date(at).toLocaleTimeString();

    expect(lineOf({at, source: "kernel", line: "booted"})).toBe(`${when} [kernel] booted`);
    expect(lineOf({at, stream: "stderr", line: "oops"})).toBe(`${when} [stderr] oops`);
    expect(lineOf({at: "", line: "bare"})).toBe("bare");
  });
});
