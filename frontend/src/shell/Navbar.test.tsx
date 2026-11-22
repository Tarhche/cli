import {screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {describe, expect, it, vi} from "vitest";
import type {ResourceDefinition} from "../api/services";
import {render} from "../test/render";
import {Navbar} from "./Navbar";

const articles = {name: "article", title: "Articles", section: "Content", audiences: ["mine", "everyone"]} as unknown as ResourceDefinition;
const users = {name: "user", title: "Users", section: "Administration", audiences: ["everyone"]} as unknown as ResourceDefinition;

describe("Navbar", () => {
  it("offers the workload's pages, then a section of resources each", () => {
    render(<Navbar view={{page: "vm", uuid: "v-1"}} definitions={[articles, users]} navigate={vi.fn()} />);

    for (const label of ["Containers", "VMs", "Images", "Volumes", "Networks", "Stacks", "Snapshots", "Port forwards", "Content", "Articles", "Administration", "Users"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }

    expect(screen.getByText("VMs").closest("a")).toHaveAttribute("data-active", "true");
  });

  it("goes where it is asked", async () => {
    const navigate = vi.fn();
    render(<Navbar view={{page: "containers"}} definitions={[articles]} navigate={navigate} />);

    await userEvent.click(screen.getByText("Snapshots"));
    await userEvent.click(screen.getByText("Articles"));

    expect(navigate).toHaveBeenNthCalledWith(1, {page: "snapshots"});
    expect(navigate).toHaveBeenNthCalledWith(2, {page: "resource", name: "article"});
  });
});
