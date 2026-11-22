import {act, screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {onCheckForUpdates} from "../api/events";
import {Updates} from "../api/services";
import {render} from "../test/render";
import {useUpdates} from "./updates";
import {UpdatesProvider} from "./UpdatesProvider";

vi.mock("../api/services", () => ({
  Updates: {Check: vi.fn(), Upgrade: vi.fn(), Restart: vi.fn(), Version: vi.fn()},
}));

vi.mock("../api/events", () => ({
  onUpdateProgress: vi.fn(() => () => undefined),
  onCheckForUpdates: vi.fn(() => () => undefined),
}));

/** Shows the version the provider gives, and asks it to look for a newer one. */
function Asker() {
  const updates = useUpdates();

  return (
    <button type="button" onClick={updates.check}>
      {updates.version ?? "no version"}
    </button>
  );
}

describe("UpdatesProvider", () => {
  beforeEach(() => {
    vi.mocked(Updates.Version).mockReset().mockResolvedValue("0.2.0" as never);
    vi.mocked(Updates.Check).mockReset().mockReturnValue(new Promise(() => undefined) as never);
  });

  it("gives the version running, and looks for a newer one when asked", async () => {
    render(
      <UpdatesProvider>
        <Asker />
      </UpdatesProvider>,
    );

    await userEvent.click(await screen.findByRole("button", {name: "0.2.0"}));

    expect(await screen.findByText("Looking for a newer version…")).toBeInTheDocument();
    expect(Updates.Check).toHaveBeenCalledTimes(1);
  });

  it("looks for a newer version when a menu of the app's asks", async () => {
    render(
      <UpdatesProvider>
        <Asker />
      </UpdatesProvider>,
    );
    const asked = vi.mocked(onCheckForUpdates).mock.calls[0][0];

    act(() => asked());

    expect(await screen.findByText("Looking for a newer version…")).toBeInTheDocument();
  });

  it("gives no version and opens nothing outside a provider", async () => {
    render(<Asker />);

    await userEvent.click(screen.getByRole("button", {name: "no version"}));

    expect(screen.queryByText("Looking for a newer version…")).not.toBeInTheDocument();
  });
});
