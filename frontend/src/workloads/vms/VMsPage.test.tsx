import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {render} from "../../test/render";
import {abilities, access} from "../../test/access";
import {WithAccess} from "../../test/WithAccess";
import {VMsPage} from "./VMsPage";

vi.mock("../../api/services", () => ({
  Workloads: {VMs: vi.fn(), StartVM: vi.fn(), StopVM: vi.fn(), RestartVM: vi.fn(), VMDependents: vi.fn(), DeleteVM: vi.fn()},
}));

const vm = (uuid: string, name: string, state: string, docker = false) => ({
  kind: "vm",
  metadata: {uuid, name, slug: `${name}-abc`, created_at: "2026-11-22T09:00:00Z"},
  spec: {image: "ubuntu:24.04", resources: {cpus: 1, memory: 128 * 1024 * 1024, disk: 1024 ** 3}, ports: [], network: {ingress: "allow", egress: "allow"}},
  status: {state, stats: null},
  docker,
  urls: [],
});

describe("VMsPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.VMs).mockResolvedValue({items: [vm("v-1", "box", "stopped"), vm("v-2", "docker", "running", true)], pagination: {total_pages: 1, current_page: 1}} as never);
  });

  it("lists one's VMs, their kind, state and size", async () => {
    render(
      <WithAccess>
        <NavigationProvider>
          <VMsPage />
        </NavigationProvider>
      </WithAccess>,
    );

    expect(await screen.findByText("box")).toBeInTheDocument();
    expect(screen.getByText("stopped")).toBeInTheDocument();
    expect(screen.getAllByText("1 vCPU · 128 MiB · 1 GiB")).toHaveLength(2);
    expect(screen.getByText("machine")).toBeInTheDocument();
    expect(screen.getByText("docker", {selector: ".mantine-Badge-label"})).toBeInTheDocument();
  });

  it("starts a VM that is stopped, and stops one that runs", async () => {
    vi.mocked(Workloads.StartVM).mockResolvedValue(undefined as never);
    vi.mocked(Workloads.StopVM).mockResolvedValue(undefined as never);
    render(
      <WithAccess>
        <NavigationProvider>
          <VMsPage />
        </NavigationProvider>
      </WithAccess>,
    );
    await screen.findByText("box");

    await userEvent.click(screen.getByRole("button", {name: "Start"}));
    await userEvent.click(screen.getByRole("button", {name: "Stop"}));

    await waitFor(() => expect(Workloads.StopVM).toHaveBeenCalledWith("v-2"));
    expect(Workloads.StartVM).toHaveBeenCalledWith("v-1");
  });

  it("offers only what the session may do with its VMs", async () => {
    render(
      <WithAccess access={access({abilities: abilities({vms: {list: true, stop: true}})})}>
        <NavigationProvider>
          <VMsPage />
        </NavigationProvider>
      </WithAccess>,
    );
    await screen.findByText("box");

    expect(screen.queryByRole("button", {name: "Create VM"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Start"})).not.toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Stop"})).toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Restart"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Delete"})).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", {name: "Inspect"})).toHaveLength(2);
  });

  it("says when there are none", async () => {
    vi.mocked(Workloads.VMs).mockResolvedValue({items: null} as never);
    render(
      <WithAccess>
        <NavigationProvider>
          <VMsPage />
        </NavigationProvider>
      </WithAccess>,
    );

    expect(await screen.findByText("No VMs yet")).toBeInTheDocument();
  });
});
