import {describe, expect, it} from "vitest";
import {newID} from "./id";

describe("newID", () => {
  it("is a UUID where there is one to be had", () => {
    expect(newID({getRandomValues: crypto.getRandomValues.bind(crypto), randomUUID: () => "a-b-c-d-e"})).toBe("a-b-c-d-e");
  });

  it("is random hex where there is not", () => {
    const random = {getRandomValues: <T extends ArrayBufferView | null>(array: T) => {
      new Uint8Array((array as unknown as Uint8Array).buffer).fill(171);

      return array;
    }};

    expect(newID(random)).toBe("ab".repeat(16));
  });

  it("is new every time", () => {
    expect(newID()).not.toBe(newID());
  });
});
