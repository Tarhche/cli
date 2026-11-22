import {screen} from "@testing-library/react";
import {describe, expect, it, vi} from "vitest";
import type {Status} from "./api/services";
import {Gate} from "./App";
import {render} from "./test/render";
import {session, testContext} from "./test/session";
import {WithSession} from "./test/WithSession";

vi.mock("./api/services", () => ({
  Session: {LoginWithBrowser: vi.fn(), LoginWithPassword: vi.fn(), Logout: vi.fn()},
  Resources: {Definitions: vi.fn(() => Promise.resolve([]))},
  Workloads: {Containers: vi.fn(() => Promise.resolve({items: []}))},
}));

describe("Gate", () => {
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
    const status = {context: testContext, logged_in: true, method: "oauth", subject: "u-1", expires_at: "", user: {uuid: "u-1", name: "Sarah"}} as Status;

    render(
      <WithSession state={session({status})}>
        <Gate />
      </WithSession>,
    );

    expect(await screen.findByRole("heading", {name: "Containers"})).toBeInTheDocument();
    expect(screen.getByText("Sarah")).toBeInTheDocument();
    expect(await screen.findByText("No containers yet")).toBeInTheDocument();
  });
});
