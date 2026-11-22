import {screen, within} from "@testing-library/react";
import {describe, expect, it, vi} from "vitest";
import {abilities, access} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {NetworksPage} from "./NetworksPage";

const network = (uuid: string, name: string, managed: boolean) => ({
  kind: "network",
  metadata: {uuid, name},
  spec: {name: managed ? name : ""},
  status: {state: managed ? "present" : "unmanaged", name, driver: name === "host" || name === "none" ? name : "bridge"},
});

vi.mock("../../api/services", () => ({
  Workloads: {
    VMs: vi.fn(() =>
      Promise.resolve({
        items: [{kind: "vm", metadata: {uuid: "v-1", name: "dock"}, spec: {image: "docker", resources: {cpus: 1, memory: 1, disk: 1}, network: {ingress: "allow", egress: "allow"}}, status: {state: "running"}, docker: true}],
        pagination: {total_pages: 1, current_page: 1},
      }),
    ),
    Networks: vi.fn(() => Promise.resolve({items: [network("n-1", "bridge", false), network("n-2", "host", false), network("n-3", "none", false), network("n-4", "backend", true)]})),
  },
}));

/** The row a network is listed in, by the name its first cell starts with. */
function rowOf(name: string) {
  const row = screen.getAllByRole("row").find((each) => within(each).queryAllByRole("cell")[0]?.textContent?.startsWith(name));

  return within(row as HTMLElement);
}

describe("NetworksPage", () => {
  it("offers to delete a network of one's own, never one of Docker's", async () => {
    render(
      <WithAccess access={access({abilities: abilities({vms: {list: true}, networks: {list: true, delete: true}})})}>
        <NetworksPage />
      </WithAccess>,
    );
    await screen.findByText("backend");

    expect(rowOf("backend").getByRole("button", {name: "Delete"})).toBeInTheDocument();
    expect(rowOf("backend").queryByText("Docker's own")).not.toBeInTheDocument();

    for (const name of ["bridge", "host", "none"]) {
      expect(rowOf(name).queryByRole("button", {name: "Delete"})).not.toBeInTheDocument();
      expect(rowOf(name).getByText("Docker's own")).toBeInTheDocument();
    }
  });
});
