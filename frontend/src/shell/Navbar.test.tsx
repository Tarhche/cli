import {screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {describe, expect, it, vi} from "vitest";
import {abilities, access, definition} from "../test/access";
import {render} from "../test/render";
import {Navbar} from "./Navbar";

const articles = definition("article", "Content", ["mine", "everyone"]);
const users = definition("user", "Administration", ["everyone"]);

describe("Navbar", () => {
  it("offers the workload's pages, then a section of resources each", () => {
    render(<Navbar view={{page: "vm", uuid: "v-1"}} access={access({definitions: [articles, users]})} navigate={vi.fn()} />);

    for (const label of ["Containers", "VMs", "Images", "Volumes", "Networks", "Stacks", "Snapshots", "Port forwards", "Content", "article", "Administration", "user"]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }

    expect(screen.getByText("VMs").closest("a")).toHaveAttribute("data-active", "true");
  });

  it("goes where it is asked", async () => {
    const navigate = vi.fn();
    render(<Navbar view={{page: "containers"}} access={access({definitions: [articles]})} navigate={navigate} />);

    await userEvent.click(screen.getByText("Snapshots"));
    await userEvent.click(screen.getByText("article"));

    expect(navigate).toHaveBeenNthCalledWith(1, {page: "snapshots"});
    expect(navigate).toHaveBeenNthCalledWith(2, {page: "resource", name: "article"});
  });

  it("offers only the workload's pages whose listing the session may read", () => {
    render(<Navbar view={{page: "vms"}} access={access({abilities: abilities({vms: {list: true}, stacks: {list: true, create: true}})})} navigate={vi.fn()} />);

    expect(screen.getByText("VMs")).toBeInTheDocument();
    expect(screen.getByText("Stacks")).toBeInTheDocument();

    for (const label of ["Containers", "Images", "Volumes", "Networks", "Snapshots", "Port forwards"]) {
      expect(screen.queryByText(label)).not.toBeInTheDocument();
    }
  });

  it("offers no section that has nothing in it", () => {
    render(<Navbar view={{page: "resource", name: "user"}} access={access({abilities: abilities(), definitions: [users]})} navigate={vi.fn()} />);

    expect(screen.queryByText("Workloads")).not.toBeInTheDocument();
    expect(screen.queryByText("Content")).not.toBeInTheDocument();
    expect(screen.getByText("Administration")).toBeInTheDocument();
    expect(screen.getByText("user")).toBeInTheDocument();
  });
});
