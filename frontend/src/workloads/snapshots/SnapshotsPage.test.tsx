import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {SnapshotsPage} from "./SnapshotsPage";

vi.mock("../../api/services", () => ({
  Workloads: {Snapshots: vi.fn(), VMs: vi.fn(), RestoreVM: vi.fn(), RenameSnapshot: vi.fn(), DeleteSnapshot: vi.fn()},
}));

const snapshot = (uuid: string, name: string, state: string) => ({
  kind: "snapshot",
  metadata: {uuid, name},
  spec: {vm: {uuid: "v-1", name: "box"}},
  status: {state, size: 1024, disk: 2 * 1024 ** 3},
  docker: false,
});

const vm = (uuid: string, name: string, state: string, disk: number, docker = false) => ({
  kind: "vm",
  metadata: {uuid, name},
  spec: {image: "ubuntu:24.04", resources: {cpus: 1, memory: 128 * 1024 * 1024, disk}, ports: [], network: {ingress: "allow", egress: "allow"}},
  status: {state, stats: null},
  docker,
  urls: [],
});

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationProvider home={{page: "snapshots"}}>
        <SnapshotsPage />
      </NavigationProvider>
    </WithAccess>,
  );
}

/** The row a snapshot is listed in. */
function rowOf(name: string) {
  return within(screen.getByText(name).closest("tr") as HTMLElement);
}

/** What a snapshot's row offers, in its order. */
function actionsOf(name: string): string[] {
  return rowOf(name).queryAllByRole("button").map((button) => button.getAttribute("aria-label") ?? "");
}

describe("SnapshotsPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.Snapshots).mockResolvedValue({items: [snapshot("s-1", "before", "ready"), snapshot("s-2", "now", "creating")], pagination: {total_pages: 1, current_page: 1}} as never);
    vi.mocked(Workloads.VMs).mockResolvedValue({
      items: [vm("v-1", "box", "running", 1024 ** 3), vm("v-2", "spare", "stopped", 4 * 1024 ** 3), vm("v-3", "dock", "running", 8 * 1024 ** 3, true)],
      pagination: {total_pages: 1, current_page: 1},
    } as never);
    vi.mocked(Workloads.RestoreVM).mockResolvedValue(undefined as never);
  });

  it("offers what the web dashboard does, in its order, as the session may", async () => {
    renderPage({snapshots: {list: true, update: true, delete: true}, vms: {create: true, restore: true}});
    await screen.findByText("before");

    expect(actionsOf("before")).toEqual(["Restore as new VM", "Restore onto a VM", "Rename", "Delete"]);
  });

  it("restores only a snapshot that is ready", async () => {
    renderPage({snapshots: {list: true}, vms: {create: true, restore: true}});
    await screen.findByText("now");

    expect(rowOf("now").getByRole("button", {name: "Restore as new VM"})).toBeDisabled();
    expect(rowOf("now").getByRole("button", {name: "Restore onto a VM"})).toBeDisabled();
    expect(rowOf("before").getByRole("button", {name: "Restore onto a VM"})).toBeEnabled();
  });

  it("offers nothing the session may not do with its snapshots", async () => {
    renderPage({snapshots: {list: true}});
    await screen.findByText("before");

    expect(actionsOf("before")).toEqual([]);
    expect(screen.queryByText(/A snapshot is taken on its VM's page/)).not.toBeInTheDocument();
  });

  it("restores one onto a VM of its kind that can take it, saying its disk is replaced", async () => {
    renderPage({snapshots: {list: true}, vms: {restore: true}});
    await screen.findByText("before");

    await userEvent.click(rowOf("before").getByRole("button", {name: "Restore onto a VM"}));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(await within(dialog).findByRole("combobox", {name: "VM"}));

    expect(await screen.findByRole("option", {name: "box: its disk is too small"})).toHaveAttribute("data-combobox-disabled", "true");
    expect(screen.queryByRole("option", {name: /dock/})).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("option", {name: "spare"}));

    expect(within(dialog).getByText(/replaces its disk with the snapshot's\. Whatever is on its disk now is lost\./)).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", {name: "Restore"}));

    await waitFor(() => expect(Workloads.RestoreVM).toHaveBeenCalledWith("v-2", "s-1"));
  });
});
