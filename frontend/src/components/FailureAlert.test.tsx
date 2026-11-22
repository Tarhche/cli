import {screen} from "@testing-library/react";
import {describe, expect, it} from "vitest";
import {render} from "../test/render";
import {FailureAlert} from "./FailureAlert";

describe("FailureAlert", () => {
  it("says a permission denied where it happened, naming the permission", () => {
    const message = 'permission denied: this needs the "self.workload.vms.exec" permission, which none of your roles gives you';

    render(<FailureAlert failure={{message, denied: true, permission: "self.workload.vms.exec"}} title="The terminal failed" />);

    expect(screen.getByRole("alert")).toHaveTextContent("The terminal failed");
    expect(screen.getByRole("alert")).toHaveTextContent(message);
  });

  it("says nothing when nothing went wrong", () => {
    render(<FailureAlert failure={null} />);

    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
