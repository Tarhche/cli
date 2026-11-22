import {act, renderHook, waitFor} from "@testing-library/react";
import type {ReactNode} from "react";
import {describe, expect, it, vi} from "vitest";
import {SignedOutContext} from "./signedOut";
import {useQuery} from "./useQuery";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });

  return {promise, resolve, reject};
}

describe("useQuery", () => {
  it("reads what it loads", async () => {
    const {result} = renderHook(() => useQuery("k", () => Promise.resolve(42)));

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.data).toBe(42));
    expect(result.current.loading).toBe(false);
    expect(result.current.failure).toBeNull();
  });

  it("says why it could not read, and that the session is gone when it is", async () => {
    const signedOut = vi.fn();
    const wrapper = ({children}: {children: ReactNode}) => <SignedOutContext.Provider value={signedOut}>{children}</SignedOutContext.Provider>;
    const failure = new Error("gone", {cause: {message: "not logged in", logged_out: true}});

    const {result} = renderHook(() => useQuery("k", () => Promise.reject(failure)), {wrapper});

    await waitFor(() => expect(result.current.failure).toEqual({message: "not logged in", logged_out: true}));
    expect(signedOut).toHaveBeenCalledOnce();
    expect(signedOut).toHaveBeenCalledWith({message: "not logged in", logged_out: true});
    expect(result.current.loading).toBe(false);
  });

  it("keeps what it read when reading again fails", async () => {
    let fail = false;
    const {result} = renderHook(() => useQuery("k", () => (fail ? Promise.reject(new Error("down")) : Promise.resolve("read"))));
    await waitFor(() => expect(result.current.data).toBe("read"));

    fail = true;
    act(() => result.current.reload());

    await waitFor(() => expect(result.current.failure).toEqual({message: "down"}));
    expect(result.current.data).toBe("read");
  });

  it("ignores a call that was cancelled", async () => {
    const cancelled = Object.assign(new Error("cancelled"), {name: "CancelError"});
    const {result} = renderHook(() => useQuery("k", () => Promise.reject(cancelled)));

    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(result.current.failure).toBeNull();
  });

  it("reads again for another key, and drops what the first answers late", async () => {
    const first = deferred<string>();
    const second = deferred<string>();
    const loads: Record<string, Promise<string>> = {a: first.promise, b: second.promise};

    const {result, rerender} = renderHook(({key}) => useQuery(key, () => loads[key]), {initialProps: {key: "a"}});
    rerender({key: "b"});

    await act(async () => {
      second.resolve("b's");
      first.resolve("a's");
    });

    expect(result.current.data).toBe("b's");
  });

  it("reads nothing while it is not enabled", () => {
    const load = vi.fn(() => Promise.resolve(1));

    const {result} = renderHook(() => useQuery("k", load, {enabled: false}));

    expect(load).not.toHaveBeenCalled();
    expect(result.current.loading).toBe(false);
    expect(result.current.data).toBeUndefined();
  });

  it("reads again every interval", async () => {
    vi.useFakeTimers();
    try {
      const load = vi.fn(() => Promise.resolve(1));
      const {unmount} = renderHook(() => useQuery("k", load, {interval: 1000}));

      for (let tick = 0; tick < 2; tick++) {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(1000);
        });
      }

      expect(load).toHaveBeenCalledTimes(3);
      unmount();
    } finally {
      vi.useRealTimers();
    }
  });
});
