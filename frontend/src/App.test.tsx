import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Resources, type Status, Workloads} from "./api/services";
import {SignedOutContext} from "./api/signedOut";
import {Gate} from "./App";
import {abilities, definition, everything} from "./test/access";
import {render} from "./test/render";
import {session, testContext} from "./test/session";
import {WithSession} from "./test/WithSession";

vi.mock("./api/services", () => ({
  Session: {LoginWithBrowser: vi.fn(), LoginWithPassword: vi.fn(), Logout: vi.fn()},
  Resources: {Definitions: vi.fn(), List: vi.fn(() => new Promise(() => undefined))},
  Workloads: {Abilities: vi.fn(), Containers: vi.fn(() => Promise.resolve({items: []})), VMs: vi.fn(() => Promise.resolve({items: []}))},
}));

/** A session signed in to the test context, read afresh. */
function signedIn(): Status {
  return {context: testContext, logged_in: true, method: "oauth", subject: "u-1", expires_at: "", permissions: [], user: {uuid: "u-1", name: "Sarah"}} as unknown as Status;
}

describe("Gate", () => {
  beforeEach(() => {
    vi.mocked(Resources.Definitions).mockReset().mockResolvedValue([] as never);
    vi.mocked(Workloads.Abilities).mockReset().mockResolvedValue(everything() as never);
  });

  it("waits for the session to be read", () => {
    render(
      <WithSession state={session({status: undefined})}>
        <Gate />
      </WithSession>,
    );

    expect(screen.queryByRole("heading", {name: "Sign in"})).not.toBeInTheDocument();
  });

  it("shows the login while the context has no session", () => {
    render(
      <WithSession state={session()}>
        <Gate />
      </WithSession>,
    );

    expect(screen.getByRole("heading", {name: "Sign in"})).toBeInTheDocument();
  });

  it("shows the login when the session could not be read", () => {
    render(
      <WithSession state={session({status: undefined, failure: {message: "broken"}})}>
        <Gate />
      </WithSession>,
    );

    expect(screen.getByRole("heading", {name: "Sign in"})).toBeInTheDocument();
  });

  it("shows the app once it is signed in", async () => {
    render(
      <WithSession state={session({status: signedIn()})}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByRole("heading", {name: "Containers"})).toBeInTheDocument();
    expect(screen.getByText("Sarah")).toBeInTheDocument();
    expect(await screen.findByText("No containers yet")).toBeInTheDocument();
  });

  it("starts at the first page the session may see, and offers only what it may", async () => {
    vi.mocked(Workloads.Abilities).mockResolvedValue(abilities({vms: {list: true}}) as never);
    vi.mocked(Resources.Definitions).mockResolvedValue([definition("user", "Administration", ["everyone"])] as never);

    render(
      <WithSession state={session({status: signedIn()})}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByRole("heading", {name: "VMs"})).toBeInTheDocument();
    expect(screen.getByText("user")).toBeInTheDocument();
    expect(screen.queryByText("Containers")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Create VM"})).not.toBeInTheDocument();
  });

  it("says when the session may see nothing at all", async () => {
    vi.mocked(Workloads.Abilities).mockResolvedValue(abilities() as never);

    render(
      <WithSession state={session({status: signedIn()})}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByText("Your roles let you see nothing here yet")).toBeInTheDocument();
    expect(screen.queryByText("Workloads")).not.toBeInTheDocument();
  });

  it("reads what the session may do again only once its permissions change", async () => {
    const state = session({status: signedIn()});
    const {rerender} = render(
      <WithSession state={state}>
        <Gate />
      </WithSession>,
    );
    expect(await screen.findByRole("heading", {name: "Containers"})).toBeInTheDocument();

    rerender(
      <WithSession state={{...state, status: signedIn()}}>
        <Gate />
      </WithSession>,
    );

    expect(Workloads.Abilities).toHaveBeenCalledTimes(1);
  });

  it("follows what the session may do as its permissions change, without starting over", async () => {
    vi.mocked(Workloads.Abilities).mockResolvedValue(abilities({containers: {list: true}, vms: {list: true}}) as never);
    const state = session({status: signedIn()});

    const {rerender} = render(
      <WithSession state={state}>
        <Gate />
      </WithSession>,
    );
    expect(await screen.findByRole("heading", {name: "Containers"})).toBeInTheDocument();
    expect(screen.getByText("VMs")).toBeInTheDocument();

    vi.mocked(Workloads.Abilities).mockResolvedValue(abilities({vms: {list: true}}) as never);
    rerender(
      <WithSession state={{...state, status: {...signedIn(), permissions: ["self.workload.vms.index"]} as Status}}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByText("Your roles do not let you see this")).toBeInTheDocument();
    expect(screen.queryByText("Containers")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", {name: "Go to what you may see"}));

    expect(await screen.findByRole("heading", {name: "VMs"})).toBeInTheDocument();
  });

  it("says why what the session may do could not be read, and reads it again when asked", async () => {
    vi.mocked(Workloads.Abilities).mockRejectedValueOnce(new Error("x", {cause: {message: "the keychain is locked"}}));

    render(
      <WithSession state={session({status: signedIn()})}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByText("the keychain is locked")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", {name: "Try again"}));

    expect(await screen.findByRole("heading", {name: "Containers"})).toBeInTheDocument();
  });

  it("hands a session gone, as reading what it may do found it, to the login", async () => {
    const failure = {message: "the session has expired or ended: not logged in", logged_out: true, ended: true};
    vi.mocked(Workloads.Abilities).mockRejectedValue(new Error("x", {cause: failure}));
    const signedOut = vi.fn();

    render(
      <SignedOutContext.Provider value={signedOut}>
        <WithSession state={session({status: signedIn()})}>
          <Gate />
        </WithSession>
      </SignedOutContext.Provider>,
    );

    await waitFor(() => expect(signedOut).toHaveBeenCalledWith(failure));
  });
});
