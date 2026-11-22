import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Session} from "../api/services";
import {render} from "../test/render";
import {session} from "../test/session";
import {WithSession} from "../test/WithSession";
import {ContextsModal} from "./ContextsModal";

vi.mock("../api/services", () => ({
  Session: {
    Create: vi.fn(),
    Remove: vi.fn(),
  },
}));

describe("ContextsModal", () => {
  beforeEach(() => {
    vi.mocked(Session.Create).mockReset();
  });

  it("lists every context with its API, its ingress and its website", () => {
    render(
      <WithSession state={session()}>
        <ContextsModal opened onClose={() => undefined} />
      </WithSession>,
    );

    const row = screen.getByRole("row", {name: /^local current/});
    expect(within(row).getByText("http://localhost:8000")).toBeInTheDocument();
    expect(within(row).getByText("http://localhost:8030")).toBeInTheDocument();
    expect(within(row).getByText("http://localhost:3000")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", {name: "Website"})).toBeInTheDocument();
  });

  it("creates a context with its website, and reads the contexts again", async () => {
    const state = session();
    vi.mocked(Session.Create).mockResolvedValue(undefined as never);
    render(
      <WithSession state={state}>
        <ContextsModal opened onClose={() => undefined} />
      </WithSession>,
    );

    await userEvent.type(screen.getByLabelText("Name"), "staging");
    await userEvent.clear(screen.getByLabelText("API"));
    await userEvent.type(screen.getByLabelText("API"), "https://api.example.com");
    await userEvent.type(screen.getByLabelText("Website"), "https://www.example.com");
    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    await waitFor(() => expect(state.refresh).toHaveBeenCalled());
    expect(Session.Create).toHaveBeenCalledWith({name: "staging", description: "", api: "https://api.example.com", ingress: "", web: "https://www.example.com"});
  });

  it("says what is wrong with the website beside it", async () => {
    vi.mocked(Session.Create).mockRejectedValue(new Error("refused", {cause: {message: "refused", status: 400, fields: {web: `"www" is not an absolute http or https url`}}}));
    render(
      <WithSession state={session()}>
        <ContextsModal opened onClose={() => undefined} />
      </WithSession>,
    );

    await userEvent.type(screen.getByLabelText("Website"), "www");
    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    expect(await screen.findByText(`"www" is not an absolute http or https url`)).toBeInTheDocument();
  });
});
