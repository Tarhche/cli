import {screen} from "@testing-library/react";
import {describe, expect, it, vi} from "vitest";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {ImagesPage} from "./ImagesPage";

vi.mock("../../api/services", () => ({
  Workloads: {
    VMs: vi.fn(() =>
      Promise.resolve({
        items: [{kind: "vm", metadata: {uuid: "v-1", name: "dock"}, spec: {image: "docker", resources: {cpus: 1, memory: 1, disk: 1}, network: {ingress: "allow", egress: "allow"}}, status: {state: "running"}, docker: true}],
        pagination: {total_pages: 1, current_page: 1},
      }),
    ),
    Images: vi.fn(() => Promise.resolve({items: [{kind: "image", metadata: {uuid: "i-1"}, spec: {reference: "nginx:1.27"}, status: {state: "present", id: "sha256:abc", size: 1024}}]})),
  },
}));

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <ImagesPage />
    </WithAccess>,
  );
}

describe("ImagesPage", () => {
  it("offers to pull an image and delete one, as the session may", async () => {
    renderPage({vms: {list: true}, images: {list: true, create: true, delete: true}});
    await screen.findByText("nginx:1.27");

    expect(screen.getByRole("button", {name: "Pull"})).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Delete"})).toBeInTheDocument();
  });

  it("offers neither when the session may only list them", async () => {
    renderPage({vms: {list: true}, images: {list: true}});
    await screen.findByText("nginx:1.27");

    expect(screen.queryByRole("button", {name: "Pull"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Delete"})).not.toBeInTheDocument();
  });
});
