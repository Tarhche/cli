import {screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {VMPage} from "./VMPage";

vi.mock("../../api/services", () => ({
  Workloads: {VM: vi.fn(), StartVM: vi.fn(), StopVM: vi.fn(), RestartVM: vi.fn(), VMLogs: vi.fn(() => new Promise(() => undefined))},
  Forwards: {List: vi.fn(() => Promise.resolve([]))},
}));

const vm = (docker: boolean) => ({
  kind: "vm",
  metadata: {uuid: "v-1", name: "box", slug: "box-abc"},
  spec: {image: "ubuntu:24.04", resources: {cpus: 1, memory: 128 * 1024 * 1024, disk: 1024 ** 3}, ports: [8080], network: {ingress: "allow", egress: "allow"}},
  status: {state: "running", stats: null},
  docker,
  urls: [],
});

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationProvider home={{page: "vms"}}>
        <VMPage uuid="v-1" />
      </NavigationProvider>
    </WithAccess>,
  );
}

function tabs(): string[] {
  return screen.getAllByRole("tab").map((tab) => tab.textContent ?? "");
}

describe("VMPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.VM).mockResolvedValue(vm(false) as never);
  });

  it("offers every tab and action to a session that may do everything", async () => {
    renderPage({vms: {show: true, start: true, stop: true, restart: true, restore: true, delete: true, logs: true, usage: true, attach: true, exec: true, forward: true}, snapshots: {create: true}});
    await screen.findByRole("heading", {name: "box"});

    expect(tabs()).toEqual(["Overview", "Logs", "Terminal", "Stats", "Files", "Ports"]);
    expect(screen.getByRole("button", {name: "Stop"})).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Restart"})).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", {name: "More"}));

    for (const item of ["Take a snapshot", "Restore a snapshot", "Inspect", "Delete"]) {
      expect(await screen.findByRole("menuitem", {name: item})).toBeInTheDocument();
    }
  });

  it("offers only the tabs whose data the session may read, and only what it may do", async () => {
    renderPage({vms: {show: true, logs: true, restart: true}});
    await screen.findByRole("heading", {name: "box"});

    expect(tabs()).toEqual(["Overview", "Logs"]);
    expect(screen.queryByRole("button", {name: "Stop"})).not.toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Restart"})).toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "More"})).not.toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Inspect"})).toBeInTheDocument();
  });

  it("offers a Docker VM's files and ports under exec alone, and a machine's ports only under forward", async () => {
    vi.mocked(Workloads.VM).mockResolvedValue(vm(true) as never);
    const docker = renderPage({vms: {show: true, exec: true}});
    await screen.findByRole("heading", {name: "box"});

    expect(tabs()).toEqual(["Overview", "Files", "Ports"]);
    docker.unmount();

    vi.mocked(Workloads.VM).mockResolvedValue(vm(false) as never);
    renderPage({vms: {show: true, exec: true}});
    await screen.findByRole("heading", {name: "box"});

    expect(tabs()).toEqual(["Overview", "Files"]);
  });

  it("goes back to the overview once a tab shown may not be read any more", async () => {
    const {rerender} = renderPage({vms: {show: true, logs: true}});
    await screen.findByRole("heading", {name: "box"});
    await userEvent.click(screen.getByRole("tab", {name: "Logs"}));
    expect(screen.getByRole("tab", {name: "Logs"})).toHaveAttribute("aria-selected", "true");

    rerender(
      <WithAccess access={access({abilities: abilities({vms: {show: true}})})}>
        <NavigationProvider home={{page: "vms"}}>
          <VMPage uuid="v-1" />
        </NavigationProvider>
      </WithAccess>,
    );

    expect(screen.queryByRole("tab", {name: "Logs"})).not.toBeInTheDocument();
    expect(screen.getByRole("tab", {name: "Overview"})).toHaveAttribute("aria-selected", "true");
  });
});
