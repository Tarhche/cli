import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {render} from "../../test/render";
import {abilities, access} from "../../test/access";
import {WithAccess} from "../../test/WithAccess";
import {CreateVMPage} from "./CreateVMPage";

vi.mock("../../api/services", () => ({
  Workloads: {CreateVM: vi.fn(), Snapshots: vi.fn(), VM: vi.fn(() => new Promise(() => undefined))},
}));

describe("CreateVMPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.Snapshots).mockResolvedValue({items: [], pagination: {total_pages: 0, current_page: 1}} as never);
    vi.mocked(Workloads.CreateVM).mockReset();
    vi.mocked(Workloads.Snapshots).mockClear();
  });

  it("creates a machine as the form says, from where it starts", async () => {
    vi.mocked(Workloads.CreateVM).mockResolvedValue({metadata: {uuid: "v-1"}} as never);
    render(
      <WithAccess>
        <NavigationProvider>
          <CreateVMPage />
        </NavigationProvider>
      </WithAccess>,
    );

    await userEvent.type(screen.getByLabelText(/Name/), "box");
    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    await waitFor(() => expect(Workloads.CreateVM).toHaveBeenCalled());
    expect(vi.mocked(Workloads.CreateVM).mock.calls[0][0]).toEqual({
      name: "box",
      kind: "machine",
      image: "ubuntu:24.04",
      resources: {cpus: 1, memory: 128 * 1024 * 1024, disk: 1024 ** 3},
      ports: [],
      network: {ingress: "allow", egress: "allow"},
      persistent_disk: true,
      lifetime_seconds: 0,
      snapshot_uuid: "",
    });
  });

  it("asks a Docker VM for what one starts with", async () => {
    vi.mocked(Workloads.CreateVM).mockResolvedValue(null as never);
    render(
      <WithAccess>
        <NavigationProvider>
          <CreateVMPage />
        </NavigationProvider>
      </WithAccess>,
    );

    await userEvent.click(screen.getByText("Docker VM"));
    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    await waitFor(() => expect(Workloads.CreateVM).toHaveBeenCalled());
    expect(vi.mocked(Workloads.CreateVM).mock.calls[0][0]).toMatchObject({kind: "docker", image: "", resources: {memory: 512 * 1024 * 1024, disk: 4 * 1024 ** 3}, ports: [80, 443, 8080]});
  });

  it("makes a VM from a snapshot of the snapshot's kind", async () => {
    vi.mocked(Workloads.Snapshots).mockResolvedValue({
      items: [{kind: "snapshot", metadata: {uuid: "s-1", name: "before"}, spec: {vm: {uuid: "v-0"}}, status: {state: "ready", disk: 4 * 1024 ** 3}, docker: true}],
      pagination: {total_pages: 1, current_page: 1},
    } as never);
    vi.mocked(Workloads.CreateVM).mockResolvedValue(null as never);
    render(
      <WithAccess>
        <NavigationProvider>
          <CreateVMPage snapshot="s-1" />
        </NavigationProvider>
      </WithAccess>,
    );

    await screen.findByDisplayValue(/before/);
    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    await waitFor(() => expect(Workloads.CreateVM).toHaveBeenCalled());
    expect(vi.mocked(Workloads.CreateVM).mock.calls[0][0]).toMatchObject({kind: "docker", image: "", snapshot_uuid: "s-1"});
  });

  it("offers no snapshot to make it from when the session may not list them", async () => {
    render(
      <WithAccess access={access({abilities: abilities({vms: {create: true}})})}>
        <NavigationProvider>
          <CreateVMPage />
        </NavigationProvider>
      </WithAccess>,
    );

    expect(screen.getByRole("button", {name: "Create"})).toBeInTheDocument();
    expect(screen.queryByText("From a snapshot")).not.toBeInTheDocument();
    expect(Workloads.Snapshots).not.toHaveBeenCalled();
  });

  it("says what the API refused beside the fields it is of", async () => {
    vi.mocked(Workloads.CreateVM).mockRejectedValue(
      new Error("x", {cause: {message: "refused", status: 400, fields: {name: "the name is required", "resources.memory": "at least 128 MiB", odd: "something else"}}}),
    );
    render(
      <WithAccess>
        <NavigationProvider>
          <CreateVMPage />
        </NavigationProvider>
      </WithAccess>,
    );

    await userEvent.click(screen.getByRole("button", {name: "Create"}));

    expect(await screen.findByText("the name is required")).toBeInTheDocument();
    expect(screen.getByText("at least 128 MiB")).toBeInTheDocument();
    expect(screen.getByText("odd: something else")).toBeInTheDocument();
  });
});
