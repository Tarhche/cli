import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {ContainerPage} from "./ContainerPage";

vi.mock("../../api/services", () => ({
  Workloads: {Container: vi.fn(), StartContainer: vi.fn(), StopContainer: vi.fn(), RestartContainer: vi.fn(), DeleteContainer: vi.fn()},
}));

const container = {
  kind: "container",
  metadata: {uuid: "c-1", name: "web"},
  spec: {name: "web", image: "nginx", ports: [{container_port: 80, host_port: 8080}]},
  status: {state: "running", name: "web", id: "abc123"},
  vm_uuid: "v-1",
  vm_name: "dock",
};

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationProvider home={{page: "containers"}}>
        <ContainerPage vm="v-1" id="c-1" />
      </NavigationProvider>
    </WithAccess>,
  );
}

describe("ContainerPage", () => {
  beforeEach(() => {
    vi.mocked(Workloads.Container).mockResolvedValue(container as never);
  });

  it("offers its terminal, files and ports under its Docker VM's exec, and its log and stats as the session may read them", async () => {
    renderPage({containers: {show: true, logs: true, usage: true, exec: true, stop: true, restart: true, delete: true}});
    await screen.findByRole("heading", {name: "web"});

    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Overview", "Logs", "Terminal", "Stats", "Files", "Ports"]);
    expect(screen.getByRole("button", {name: "Stop"})).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", {name: "More"}));

    expect(await screen.findByRole("menuitem", {name: "Delete"})).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", {name: "Inspect"})).not.toBeInTheDocument();
  });

  it("shows everything Docker says of it once asked, and folds it again", async () => {
    renderPage({containers: {show: true}});
    await screen.findByRole("heading", {name: "web"});
    const show = screen.getByRole("button", {name: "Show everything Docker says about it"});
    expect(show).toHaveAttribute("aria-expanded", "false");

    await userEvent.click(show);

    await waitFor(() => expect(screen.getByText(/"id": "abc123"/)).toBeVisible());
    await userEvent.click(screen.getByRole("button", {name: "Hide the details"}));
    expect(screen.getByRole("button", {name: "Show everything Docker says about it"})).toHaveAttribute("aria-expanded", "false");
  });

  it("offers nothing the session may not do with it", async () => {
    renderPage({containers: {show: true}});
    await screen.findByRole("heading", {name: "web"});

    expect(screen.getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Overview"]);
    expect(screen.queryByRole("button", {name: "Stop"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Restart"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "More"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Inspect"})).not.toBeInTheDocument();
  });
});
