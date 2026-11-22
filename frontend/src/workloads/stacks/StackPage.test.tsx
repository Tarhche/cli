import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationContext, type View} from "../../shell/navigation";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {StackPage} from "./StackPage";

vi.mock("../../api/services", () => ({
  Workloads: {Stack: vi.fn(), VM: vi.fn(), StartStack: vi.fn(), StopStack: vi.fn(), RestartStack: vi.fn(), DeleteStack: vi.fn()},
}));

const stack = (state: string, reason?: string) => ({
  kind: "stack",
  metadata: {uuid: "s-1", name: "shop", slug: "shop-abc", created_at: "2026-11-22T09:00:00Z", refs: [{kind: "vm", uuid: "v-1"}]},
  spec: {vm: {}, compose: "services:\n  web:\n    image: nginx\n"},
  status: {
    state,
    reason,
    services: [
      {name: "web", state: "running", containers: [{id: "c-1", name: "shop-abc_web", state: "running"}]},
      {name: "db", state: "running", containers: [{id: "c-2", name: "shop-abc_db", state: "running"}]},
    ],
  },
  vm_name: "dock",
});

const vm = {kind: "vm", metadata: {uuid: "v-1", name: "dock"}, spec: {}, status: {state: "running", stats: null}, docker: true, urls: []};

function renderPage(allowed: Allowed, navigate: (view: View) => void = vi.fn()) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationContext.Provider value={{view: {page: "stack", uuid: "s-1"}, home: {page: "stacks"}, navigate, back: vi.fn()}}>
        <StackPage uuid="s-1" />
      </NavigationContext.Provider>
    </WithAccess>,
  );
}

describe("StackPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.Stack).mockResolvedValue(stack("running") as never);
    vi.mocked(Workloads.VM).mockResolvedValue(vm as never);
  });

  it("shows where it is, its VM, what it names what it makes, and its containers", async () => {
    const navigate = vi.fn();
    renderPage({stacks: {show: true}, vms: {show: true}, containers: {show: true}}, navigate);
    await screen.findByRole("heading", {name: "shop"});

    expect(screen.getByText("what it makes is named shop-abc_…")).toBeInTheDocument();
    expect(screen.getAllByRole("row").map((row) => within(row).queryAllByRole("cell").map((cell) => cell.textContent))).toContainEqual(["shop-abc_web", "web", "running"]);

    await userEvent.click(await screen.findByText("dock"));
    expect(navigate).toHaveBeenCalledWith({page: "vm", uuid: "v-1"});

    await userEvent.click(screen.getByText("shop-abc_db"));
    expect(navigate).toHaveBeenCalledWith({page: "container", vm: "v-1", id: "c-2"});
  });

  it("links to neither its VM nor its containers when the session may not read them", async () => {
    renderPage({stacks: {show: true}});
    await screen.findByRole("heading", {name: "shop"});

    expect(screen.getByText("dock").closest("a, button")).toBeNull();
    expect(screen.getByText("shop-abc_web").closest("a, button")).toBeNull();
    expect(Workloads.VM).not.toHaveBeenCalled();
  });

  it("keeps its compose file folded until it is asked for", async () => {
    renderPage({stacks: {show: true}});
    await screen.findByRole("heading", {name: "shop"});

    const show = screen.getByRole("button", {name: "Show"});
    expect(show).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(show);

    await waitFor(() => expect(screen.getByText(/image: nginx/)).toBeVisible());
    expect(screen.getByRole("button", {name: "Hide"})).toHaveAttribute("aria-expanded", "true");
  });

  it("says why it failed", async () => {
    vi.mocked(Workloads.Stack).mockResolvedValue(stack("failed", "the image could not be pulled") as never);
    renderPage({stacks: {show: true}});

    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText("Why it failed")).toBeInTheDocument();
    expect(within(alert).getByText("the image could not be pulled")).toBeInTheDocument();
  });

  it("is stopped and restarted once that is confirmed, and deleted from its menu, as the session may", async () => {
    renderPage({stacks: {show: true, start: true, stop: true, restart: true, delete: true}});
    await screen.findByRole("heading", {name: "shop"});

    expect(screen.queryByRole("button", {name: "Start"})).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Stop"}));
    const question = await screen.findByRole("dialog");
    expect(within(question).getByText('Stop "shop"? Every container in it is stopped.')).toBeInTheDocument();
    await userEvent.click(within(question).getByRole("button", {name: "Stop"}));
    await waitFor(() => expect(Workloads.StopStack).toHaveBeenCalledWith("s-1"));

    await userEvent.click(screen.getByRole("button", {name: "More"}));
    expect(await screen.findByRole("menuitem", {name: "Delete"})).toBeInTheDocument();
  });

  it("offers nothing the session may not do with it", async () => {
    renderPage({stacks: {show: true}});
    await screen.findByRole("heading", {name: "shop"});

    expect(screen.queryByRole("button", {name: "Stop"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Restart"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "More"})).not.toBeInTheDocument();
  });
});
