import {screen} from "@testing-library/react";
import {describe, expect, it, vi} from "vitest";
import {NavigationProvider} from "../../shell/NavigationProvider";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {SnapshotsPage} from "./SnapshotsPage";

vi.mock("../../api/services", () => ({
  Workloads: {
    Snapshots: vi.fn(() =>
      Promise.resolve({
        items: [{kind: "snapshot", metadata: {uuid: "s-1", name: "before"}, spec: {vm: {uuid: "v-1", name: "box"}}, status: {state: "ready", size: 1024}, docker: false}],
        pagination: {total_pages: 1, current_page: 1},
      }),
    ),
  },
}));

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <NavigationProvider>
        <SnapshotsPage />
      </NavigationProvider>
    </WithAccess>,
  );
}

describe("SnapshotsPage", () => {
  it("offers to make a VM from a snapshot, rename it and delete it, as the session may", async () => {
    renderPage({snapshots: {list: true, update: true, delete: true}, vms: {create: true}});
    await screen.findByText("before");

    expect(screen.getByRole("button", {name: "Make a VM from it"})).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Rename"})).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Delete"})).toBeInTheDocument();
  });

  it("offers nothing the session may not do with its snapshots", async () => {
    renderPage({snapshots: {list: true}});
    await screen.findByText("before");

    expect(screen.queryByRole("button", {name: "Make a VM from it"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Rename"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Delete"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Inspect"})).not.toBeInTheDocument();
    expect(screen.queryByText(/A snapshot is taken on its VM's page/)).not.toBeInTheDocument();
  });
});
