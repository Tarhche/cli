import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationContext, type View} from "../../shell/navigation";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {StacksPage} from "./StacksPage";

vi.mock("../../api/services", () => ({
  Workloads: {Stacks: vi.fn(), StartStack: vi.fn(), StopStack: vi.fn(), RestartStack: vi.fn(), DeleteStack: vi.fn()},
}));

const stack = {
  kind: "stack",
  metadata: {uuid: "s-1", name: "shop", slug: "shop-abc"},
  spec: {vm: {uuid: "v-1"}, compose: "services: {}"},
  status: {state: "running", services: [{name: "web", state: "running"}]},
  vm_name: "dock",
};

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationProvider home={{page: "stacks"}}>
        <StacksPage />
      </NavigationProvider>
    </WithAccess>,
  );
}

describe("StacksPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.Stacks).mockResolvedValue({items: [stack], pagination: {total_pages: 1, current_page: 1}} as never);
  });

  it("stops a stack and restarts one once that is confirmed", async () => {
    renderPage({stacks: {list: true, stop: true, restart: true}});
    await screen.findByText("shop");

    await userEvent.click(screen.getByRole("button", {name: "Stop"}));
    const stop = await screen.findByRole("dialog");
    expect(within(stop).getByText('Stop "shop"? Every container in it is stopped.')).toBeInTheDocument();
    await userEvent.click(within(stop).getByRole("button", {name: "Stop"}));
    await waitFor(() => expect(Workloads.StopStack).toHaveBeenCalledWith("s-1"));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", {name: "Restart"}));
    const restart = await screen.findByRole("dialog");
    expect(within(restart).getByText('Restart "shop"? Every container in it is restarted.')).toBeInTheDocument();
    await userEvent.click(within(restart).getByRole("button", {name: "Cancel"}));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(Workloads.RestartStack).not.toHaveBeenCalled();
  });

  it("opens a stack's page from its row when the session may read it", async () => {
    const navigate = vi.fn<(view: View) => void>();
    const {unmount} = render(
      <WithAccess access={access({abilities: abilities({stacks: {list: true, show: true}})})}>
        <NavigationContext.Provider value={{view: {page: "stacks"}, home: {page: "stacks"}, navigate, back: vi.fn()}}>
          <StacksPage />
        </NavigationContext.Provider>
      </WithAccess>,
    );

    await userEvent.click(await screen.findByText("shop"));
    expect(navigate).toHaveBeenCalledWith({page: "stack", uuid: "s-1"});
    unmount();

    navigate.mockClear();
    render(
      <WithAccess access={access({abilities: abilities({stacks: {list: true}})})}>
        <NavigationContext.Provider value={{view: {page: "stacks"}, home: {page: "stacks"}, navigate, back: vi.fn()}}>
          <StacksPage />
        </NavigationContext.Provider>
      </WithAccess>,
    );

    await userEvent.click(await screen.findByText("shop"));
    expect(navigate).not.toHaveBeenCalled();
  });

  it("offers only what the session may do with its stacks", async () => {
    renderPage({stacks: {list: true}});
    await screen.findByText("shop");

    expect(screen.queryByRole("button", {name: "Deploy stack"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Stop"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Restart"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Delete"})).not.toBeInTheDocument();
  });
});
