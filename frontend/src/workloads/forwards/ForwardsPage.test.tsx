import {screen} from "@testing-library/react";
import {describe, expect, it, vi} from "vitest";
import {abilities, access, type Allowed} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {ForwardsPage} from "./ForwardsPage";

vi.mock("../../api/services", () => ({
  Forwards: {List: vi.fn(() => Promise.resolve([]))},
  Workloads: {VMs: vi.fn(() => Promise.resolve({items: []})), Containers: vi.fn(() => Promise.resolve({items: []}))},
}));

function renderPage(allowed: Allowed) {
  return render(
    <WithAccess access={access({abilities: abilities(allowed)})}>
      <ForwardsPage />
    </WithAccess>,
  );
}

describe("ForwardsPage", () => {
  it("offers to forward a port of a VM or of a container, as the session may", () => {
    renderPage({vms: {list: true, forward: true}, containers: {list: true, exec: true}});

    expect(screen.getByText("Of a VM")).toBeInTheDocument();
    expect(screen.getByText("Of a container")).toBeInTheDocument();
  });

  it("offers only what the session may forward a port of", () => {
    renderPage({vms: {list: true}, containers: {list: true, exec: true}});

    expect(screen.queryByText("Of a VM")).not.toBeInTheDocument();
    expect(screen.getByText("Container", {selector: "label"})).toBeInTheDocument();
  });
});
