import {describe, expect, it, vi} from "vitest";
import {typed} from "./bytes";
import {WriteQueue} from "./writeQueue";

const decoder = new TextDecoder();

describe("WriteQueue", () => {
  it("sends one write at a time, in order, what was typed meanwhile together", async () => {
    const sent: string[] = [];
    let release: () => void = () => undefined;
    const send = vi.fn(
      (data: Uint8Array) =>
        new Promise<void>((resolve) => {
          sent.push(decoder.decode(data));
          release = resolve;
        }),
    );
    const queue = new WriteQueue(send, () => undefined);

    queue.push(typed("l"));
    queue.push(typed("s"));
    queue.push(typed(" -la"));

    expect(sent).toEqual(["l"]);

    release();
    await vi.waitFor(() => expect(sent).toEqual(["l", "s -la"]));

    release();
    await vi.waitFor(() => expect(queue.idle).toBe(true));
    expect(send).toHaveBeenCalledTimes(2);
  });

  it("says why a write failed, and sends what follows", async () => {
    const failed = vi.fn();
    const send = vi.fn().mockRejectedValueOnce(new Error("lost")).mockResolvedValue(undefined);
    const queue = new WriteQueue(send, failed);

    queue.push(typed("a"));
    await vi.waitFor(() => expect(queue.idle).toBe(true));
    queue.push(typed("b"));
    await vi.waitFor(() => expect(queue.idle).toBe(true));

    expect(failed).toHaveBeenCalledWith(new Error("lost"));
    expect(send).toHaveBeenCalledTimes(2);
  });

  it("sends nothing of nothing", () => {
    const send = vi.fn(() => Promise.resolve());
    const queue = new WriteQueue(send, () => undefined);

    queue.push(new Uint8Array());

    expect(send).not.toHaveBeenCalled();
    expect(queue.idle).toBe(true);
  });
});
