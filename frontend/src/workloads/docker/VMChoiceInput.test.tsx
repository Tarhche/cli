import {screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {describe, expect, it, vi} from "vitest";
import {Workloads} from "../../api/services";
import {abilities, access} from "../../test/access";
import {render} from "../../test/render";
import {WithAccess} from "../../test/WithAccess";
import {VMChoiceInput} from "./VMChoiceInput";

vi.mock("../../api/services", () => ({
  Workloads: {VMs: vi.fn(() => Promise.resolve({items: [], pagination: {total_pages: 0, current_page: 1}}))},
}));

describe("VMChoiceInput", () => {
  it("offers a Docker VM of one's own, or a new one", () => {
    render(
      <WithAccess>
        <VMChoiceInput value={{fresh: true, vm: null, name: ""}} onChange={vi.fn()} />
      </WithAccess>,
    );

    expect(screen.getByText("A Docker VM of mine")).toBeInTheDocument();
    expect(screen.getByText("A new Docker VM")).toBeInTheDocument();
  });

  it("offers only a new one when the session may not list its VMs to choose among", async () => {
    const onChange = vi.fn();
    render(
      <WithAccess access={access({abilities: abilities({containers: {create: true}})})}>
        <VMChoiceInput value={{fresh: true, vm: null, name: ""}} onChange={onChange} />
      </WithAccess>,
    );

    expect(screen.queryByText("A Docker VM of mine")).not.toBeInTheDocument();
    expect(Workloads.VMs).not.toHaveBeenCalled();

    await userEvent.type(screen.getByLabelText("Runs in a new Docker VM"), "d");

    expect(onChange).toHaveBeenLastCalledWith({fresh: true, vm: null, name: "d"});
  });
});
