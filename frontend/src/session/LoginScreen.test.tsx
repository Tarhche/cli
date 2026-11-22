import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Session} from "../api/services";
import {render} from "../test/render";
import {session} from "../test/session";
import {WithSession} from "../test/WithSession";
import {LoginScreen} from "./LoginScreen";

vi.mock("../api/services", () => ({
  Session: {
    LoginWithBrowser: vi.fn(),
    LoginWithPassword: vi.fn(),
    Contexts: vi.fn(),
    Status: vi.fn(),
    Use: vi.fn(),
    Create: vi.fn(),
    Remove: vi.fn(),
  },
}));

describe("LoginScreen", () => {
  beforeEach(() => {
    vi.mocked(Session.LoginWithPassword).mockReset();
    vi.mocked(Session.LoginWithBrowser).mockReset();
  });

  it("asks to sign in to the context the app talks to", () => {
    render(
      <WithSession state={session()}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.getByRole("heading", {name: "Sign in"})).toBeInTheDocument();
    expect(screen.getByText("to local, http://localhost:8000")).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Sign in with browser"})).toBeInTheDocument();
  });

  it("signs in with a password, and reads the session again", async () => {
    const state = session();
    vi.mocked(Session.LoginWithPassword).mockResolvedValue(undefined as never);
    render(
      <WithSession state={state}>
        <LoginScreen />
      </WithSession>,
    );

    await userEvent.type(screen.getByLabelText("Username or email"), "sarah");
    await userEvent.type(screen.getByLabelText("Password"), "secret");
    await userEvent.click(screen.getByRole("button", {name: "Sign in"}));

    await waitFor(() => expect(state.refresh).toHaveBeenCalled());
    expect(Session.LoginWithPassword).toHaveBeenCalledWith("sarah", "secret");
  });

  it("says what the API refused beside each field, and the rest above", async () => {
    vi.mocked(Session.LoginWithPassword).mockRejectedValue(
      new Error("refused", {cause: {message: "refused", status: 400, fields: {identity: "this field is required", device: "odd"}}}),
    );
    render(
      <WithSession state={session()}>
        <LoginScreen />
      </WithSession>,
    );

    await userEvent.click(screen.getByRole("button", {name: "Sign in"}));

    expect(await screen.findByText("this field is required")).toBeInTheDocument();
    expect(screen.getByText("device: odd")).toBeInTheDocument();
  });

  it("waits for the browser, which can be given up on", async () => {
    const cancel = vi.fn();
    vi.mocked(Session.LoginWithBrowser).mockReturnValue(Object.assign(new Promise<void>(() => undefined), {cancel}) as never);
    render(
      <WithSession state={session()}>
        <LoginScreen />
      </WithSession>,
    );

    await userEvent.click(screen.getByRole("button", {name: "Sign in with browser"}));

    expect(await screen.findByText("Waiting for your browser…")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Cancel"}));
    expect(cancel).toHaveBeenCalled();
  });

  it("says the session expired, when it did", () => {
    render(
      <WithSession state={session({ended: {context: "local", blocked: false, reason: ""}})}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.getByText("Your session has expired")).toBeInTheDocument();
    expect(screen.queryByText("Your account is blocked")).not.toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Sign in with browser"})).toBeInTheDocument();
  });

  it("says the account is blocked, for the reason the estate gave", () => {
    render(
      <WithSession state={session({ended: {context: "local", blocked: true, reason: "The user is banned."}})}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.getByText("Your account is blocked")).toBeInTheDocument();
    expect(screen.getByText(/The user is banned\./)).toBeInTheDocument();
    expect(screen.queryByText("Your session has expired")).not.toBeInTheDocument();
  });

  it("says the account is blocked when the estate gave no reason", () => {
    render(
      <WithSession state={session({ended: {context: "local", blocked: true, reason: ""}})}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.getByText(/The estate refuses every request of yours/)).toBeInTheDocument();
  });

  it("says nothing of how another context's session ended", () => {
    render(
      <WithSession state={session({ended: {context: "default", blocked: true, reason: "The user is banned."}})}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.queryByText("Your account is blocked")).not.toBeInTheDocument();
  });

  it("says why the session could not be read", () => {
    render(
      <WithSession state={session({failure: {message: "the keychain is locked"}})}>
        <LoginScreen />
      </WithSession>,
    );

    expect(screen.getByText("the keychain is locked")).toBeInTheDocument();
  });
});
