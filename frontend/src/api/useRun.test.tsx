import {act, renderHook} from "@testing-library/react";
import type {ReactNode} from "react";
import {describe, expect, it, vi} from "vitest";
import {SignedOutContext} from "./signedOut";
import {useRun} from "./useRun";

describe("useRun", () => {
  it("does what it is asked, answering what it answered", async () => {
    const action = vi.fn((a: number, b: number) => Promise.resolve(a + b));
    const {result} = renderHook(() => useRun(action));

    let outcome;
    await act(async () => {
      outcome = await result.current.run(1, 2);
    });

    expect(outcome).toEqual({done: true, value: 3});
    expect(action).toHaveBeenCalledWith(1, 2);
    expect(result.current.pending).toBe(false);
    expect(result.current.failure).toBeNull();
  });

  it("remembers why it failed until reset, and says the session is gone when it is", async () => {
    const signedOut = vi.fn();
    const wrapper = ({children}: {children: ReactNode}) => <SignedOutContext.Provider value={signedOut}>{children}</SignedOutContext.Provider>;
    const {result} = renderHook(() => useRun(() => Promise.reject(new Error("x", {cause: {message: "gone", logged_out: true}}))), {wrapper});

    let outcome;
    await act(async () => {
      outcome = await result.current.run();
    });

    expect(outcome).toEqual({done: false});
    expect(result.current.failure).toEqual({message: "gone", logged_out: true});
    expect(signedOut).toHaveBeenCalledOnce();

    act(() => result.current.reset());
    expect(result.current.failure).toBeNull();
  });

  it("takes a call that was cancelled for no failure", async () => {
    const {result} = renderHook(() => useRun(() => Promise.reject(Object.assign(new Error("x"), {name: "CancelError"}))));

    await act(async () => {
      await result.current.run();
    });

    expect(result.current.failure).toBeNull();
  });
});
