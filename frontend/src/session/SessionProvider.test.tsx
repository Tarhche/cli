import {act, render, screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {Session, type Status} from "../api/services";
import {useSignedOut} from "../api/signedOut";
import {testContext} from "../test/session";
import {FOCUS_PAUSE, SESSION_POLL, useSession} from "./session";
import {SessionProvider} from "./SessionProvider";

vi.mock("../api/services", () => ({
  Session: {Status: vi.fn(), Contexts: vi.fn(), Use: vi.fn()},
}));

/** A session as reading it answers. */
function status(state: Partial<Status> = {}): Status {
  return {context: testContext, logged_in: false, expires_at: "", permissions: [], ...state} as Status;
}

/** Says what the session is, and signs out as a call that failed would. */
function Consumer() {
  const session = useSession();
  const signedOut = useSignedOut();
  const ended = session.ended;

  return (
    <>
      <p>{session.status ? (session.status.logged_in ? "signed in" : "signed out") : "reading"}</p>
      <p>{ended ? `${ended.context}: ${ended.blocked ? "blocked" : "expired"}${ended.reason ? `, ${ended.reason}` : ""}` : "nothing ended"}</p>
      <p>{session.failure?.message ?? "no failure"}</p>
      <button type="button" onClick={() => signedOut({message: "refused", logged_out: true, blocked: true, reason: "The user is banned."})}>
        refused
      </button>
      <button type="button" onClick={() => signedOut({message: "gone", logged_out: true})}>
        gone
      </button>
      <button type="button" onClick={() => void session.refresh()}>
        read again
      </button>
    </>
  );
}

describe("SessionProvider", () => {
  beforeEach(() => {
    vi.mocked(Session.Status).mockReset().mockResolvedValue(status({logged_in: true}) as never);
    vi.mocked(Session.Contexts).mockReset().mockResolvedValue([testContext] as never);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("reads the session and the contexts", async () => {
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );

    expect(await screen.findByText("signed in")).toBeInTheDocument();
    expect(Session.Contexts).toHaveBeenCalled();
  });

  it("reads the session again every so often while there is one, for what its renewed token says", async () => {
    vi.useFakeTimers();
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(Session.Status).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(SESSION_POLL);
    });

    expect(Session.Status).toHaveBeenCalledTimes(2);
  });

  it("does not read it again while there is none", async () => {
    vi.useFakeTimers();
    vi.mocked(Session.Status).mockResolvedValue(status() as never);
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );

    await act(async () => {
      await vi.advanceTimersByTimeAsync(SESSION_POLL * 2);
    });

    expect(Session.Status).toHaveBeenCalledTimes(1);
  });

  it("reads it again as the window comes back to the front, but not right after it read it", async () => {
    vi.useFakeTimers();
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    await act(async () => {
      window.dispatchEvent(new Event("focus"));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(Session.Status).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(FOCUS_PAUSE);
      window.dispatchEvent(new Event("focus"));
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(Session.Status).toHaveBeenCalledTimes(2);
  });

  it("keeps why the session ended, as reading it said, until somebody signs in", async () => {
    vi.mocked(Session.Status).mockResolvedValue(status({ended: true}) as never);
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );
    expect(await screen.findByText("local: expired")).toBeInTheDocument();

    vi.mocked(Session.Status).mockResolvedValue(status() as never);
    await userEvent.click(screen.getByRole("button", {name: "read again"}));
    await waitFor(() => expect(Session.Status).toHaveBeenCalledTimes(2));
    expect(screen.getByText("local: expired")).toBeInTheDocument();

    vi.mocked(Session.Status).mockResolvedValue(status({logged_in: true}) as never);
    await userEvent.click(screen.getByRole("button", {name: "read again"}));

    expect(await screen.findByText("nothing ended")).toBeInTheDocument();
  });

  it("keeps that the account is blocked, and why, as reading the session said", async () => {
    vi.mocked(Session.Status).mockResolvedValue(status({blocked: true, reason: "The user is banned."}) as never);

    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );

    expect(await screen.findByText("local: blocked, The user is banned.")).toBeInTheDocument();
  });

  it("keeps why the session ended as a call that failed said it, and reads it again", async () => {
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );
    await screen.findByText("signed in");
    vi.mocked(Session.Status).mockResolvedValue(status() as never);

    await userEvent.click(screen.getByRole("button", {name: "refused"}));

    expect(await screen.findByText("signed out")).toBeInTheDocument();
    expect(screen.getByText("local: blocked, The user is banned.")).toBeInTheDocument();
    expect(Session.Status).toHaveBeenCalledTimes(2);
  });

  it("reads the session again when a call says it is gone, saying nothing it was not told", async () => {
    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );
    await screen.findByText("signed in");
    vi.mocked(Session.Status).mockResolvedValue(status() as never);

    await userEvent.click(screen.getByRole("button", {name: "gone"}));

    expect(await screen.findByText("signed out")).toBeInTheDocument();
    expect(screen.getByText("nothing ended")).toBeInTheDocument();
  });

  it("says why the session could not be read", async () => {
    vi.mocked(Session.Status).mockRejectedValue(new Error("x", {cause: {message: "the keychain is locked"}}));

    render(
      <SessionProvider>
        <Consumer />
      </SessionProvider>,
    );

    expect(await screen.findByText("the keychain is locked")).toBeInTheDocument();
  });
});
