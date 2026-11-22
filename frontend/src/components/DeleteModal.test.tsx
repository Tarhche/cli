import {screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {describe, expect, it, vi} from "vitest";
import {render} from "../test/render";
import {DeleteModal} from "./DeleteModal";

describe("DeleteModal", () => {
  it("says what needs it, and deletes it with what needs it when asked", async () => {
    const onDelete = vi.fn(() => Promise.resolve());
    const onDeleted = vi.fn();
    const onClose = vi.fn();
    const dependents = () => Promise.resolve({dependents: [{kind: "stack", uuid: "k-1", name: "site", recreates: true}], recreated: true});

    render(<DeleteModal opened onClose={onClose} title="Delete VM" name="box" dependents={dependents} force="Delete it though it runs" onDelete={onDelete} onDeleted={onDeleted} />);

    expect(await screen.findByText(/stack site, which makes it again/)).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText("Delete what needs it too"));
    await userEvent.click(screen.getByLabelText("Delete it though it runs"));
    await userEvent.click(screen.getByRole("button", {name: "Delete"}));

    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
    expect(onDelete).toHaveBeenCalledWith({cascade: true, force: true});
    expect(onClose).toHaveBeenCalled();
  });

  it("says why it could not be deleted, and stays open", async () => {
    const onClose = vi.fn();
    const onDelete = vi.fn(() => Promise.reject(new Error("x", {cause: {message: "it is in use"}})));

    render(<DeleteModal opened onClose={onClose} title="Delete image" name="nginx" onDelete={onDelete} />);
    await userEvent.click(screen.getByRole("button", {name: "Delete"}));

    expect(await screen.findByText("it is in use")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });
});
