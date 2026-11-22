import {act, screen, waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {Browser} from "@wailsio/runtime";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {onUpdateProgress} from "../api/events";
import {type Update, type UpdateProgress, type Upgraded, Updates} from "../api/services";
import {render} from "../test/render";
import {UpdateDialog} from "./UpdateDialog";

vi.mock("../api/services", () => ({
  Updates: {Check: vi.fn(), Upgrade: vi.fn(), Restart: vi.fn(), Version: vi.fn()},
}));

vi.mock("../api/events", () => ({
  onUpdateProgress: vi.fn(() => () => undefined),
  onCheckForUpdates: vi.fn(() => () => undefined),
}));

/** What looking for a newer release found, as a test says. */
function found(update: Partial<Update> = {}): Update {
  return {
    current: "0.2.0",
    available: true,
    latest: {
      version: "0.3.0",
      name: "v0.3.0",
      notes: "## Changelog\n### Features\n* 1a2b3c4 Feat: upgrades itself",
      url: "https://github.com/Tarhche/cli/releases/tag/v0.3.0",
      published_at: new Date(Date.now() - 2 * 24 * 3600 * 1000).toISOString(),
    },
    target: "/usr/local/bin/tarhche",
    ...update,
  } as Update;
}

/** A call a test answers when it likes, which may be cancelled. */
function deferred<T>() {
  let resolve: (value: T) => void = () => undefined;
  let reject: (reason: unknown) => void = () => undefined;
  const promise = new Promise<T>((resolved, rejected) => {
    resolve = resolved;
    reject = rejected;
  }) as Promise<T> & {cancel: () => void};
  promise.cancel = vi.fn();

  return {promise, resolve, reject};
}

/** Hears what the dialog listens to of an upgrade's progress. */
function progressListener(): (progress: UpdateProgress) => void {
  const calls = vi.mocked(onUpdateProgress).mock.calls;

  return calls[calls.length - 1][0];
}

function renderDialog(onClose = vi.fn()) {
  render(<UpdateDialog opened onClose={onClose} />);

  return onClose;
}

describe("UpdateDialog", () => {
  beforeEach(() => {
    vi.mocked(Updates.Check).mockReset();
    vi.mocked(Updates.Upgrade).mockReset();
    vi.mocked(Updates.Restart).mockReset();
  });

  it("looks for a newer version as it opens", () => {
    vi.mocked(Updates.Check).mockReturnValue(new Promise(() => undefined) as never);

    renderDialog();

    expect(screen.getByText("Looking for a newer version…")).toBeInTheDocument();
  });

  it("says when the version running is the latest", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found({available: false}) as never);
    const onClose = renderDialog();

    expect(await screen.findByText("Tarhche 0.2.0 is the latest version.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Close"}));
    expect(onClose).toHaveBeenCalled();
  });

  it("shows a newer release, what it says of itself, and what upgrading replaces", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    renderDialog();

    expect(screen.getByRole("button", {name: "Close the dialog"})).toBeInTheDocument();

    expect(await screen.findByText("Tarhche 0.3.0 is available — you have 0.2.0.")).toBeInTheDocument();
    expect(screen.getByText(/Released 2 days ago/)).toBeInTheDocument();
    expect(screen.getByRole("heading", {name: "Features"})).toBeInTheDocument();
    expect(screen.getByText("1a2b3c4 Feat: upgrades itself")).toBeInTheDocument();
    expect(screen.getByText("Upgrading replaces /usr/local/bin/tarhche.")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("link", {name: "the release on GitHub"}));
    expect(vi.mocked(Browser.OpenURL)).toHaveBeenCalledWith("https://github.com/Tarhche/cli/releases/tag/v0.3.0");
  });

  it("says why a build cannot upgrade itself, and offers no upgrade", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found({target: undefined, unsupported: "this build of tarhche cannot upgrade itself: pull a newer image of ghcr.io/tarhche/cli instead"}) as never);
    renderDialog();

    expect(await screen.findByText("This build cannot upgrade itself")).toBeInTheDocument();
    expect(screen.getByText(/pull a newer image of ghcr.io\/tarhche\/cli instead/)).toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Upgrade"})).not.toBeInTheDocument();
  });

  it("says why it could not look, and looks again", async () => {
    vi.mocked(Updates.Check)
      .mockRejectedValueOnce(new Error("refused", {cause: {message: "GitHub is unreachable"}}))
      .mockResolvedValueOnce(found({available: false}) as never);
    renderDialog();

    expect(await screen.findByText("GitHub is unreachable")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Try again"}));

    expect(await screen.findByText("Tarhche 0.2.0 is the latest version.")).toBeInTheDocument();
    expect(Updates.Check).toHaveBeenCalledTimes(2);
  });

  it("downloads the release, showing how much arrived, and restarts into it", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    const upgrading = deferred<Upgraded>();
    vi.mocked(Updates.Upgrade).mockReturnValue(upgrading.promise as never);
    vi.mocked(Updates.Restart).mockResolvedValue(undefined as never);
    renderDialog();

    await userEvent.click(await screen.findByRole("button", {name: "Upgrade"}));

    expect(Updates.Upgrade).toHaveBeenCalledWith("0.3.0");
    expect(screen.getByText("Downloading Tarhche 0.3.0…")).toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Close the dialog"})).not.toBeInTheDocument();

    act(() => progressListener()({version: "0.3.0", done: 512 * 1024, total: 1024 * 1024}));
    expect(screen.getByText("512 KiB of 1 MiB")).toBeInTheDocument();
    expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "50");

    await act(async () => upgrading.resolve({version: "0.3.0", path: "/usr/local/bin/tarhche"} as Upgraded));

    expect(await screen.findByText("Upgraded to 0.3.0. Restart Tarhche to use it.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Restart now"}));

    expect(Updates.Restart).toHaveBeenCalled();
    expect(await screen.findByText("Restarting Tarhche…")).toBeInTheDocument();
  });

  it("shows how much arrived while how much there is is not known", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    vi.mocked(Updates.Upgrade).mockReturnValue(deferred<Upgraded>().promise as never);
    renderDialog();

    await userEvent.click(await screen.findByRole("button", {name: "Upgrade"}));
    act(() => progressListener()({version: "0.3.0", done: 3 * 1024 * 1024, total: 0}));

    expect(screen.getByText("3 MiB so far")).toBeInTheDocument();
  });

  it("cancels a download, and offers the upgrade again", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    const upgrading = deferred<Upgraded>();
    vi.mocked(Updates.Upgrade).mockReturnValue(upgrading.promise as never);
    renderDialog();

    await userEvent.click(await screen.findByRole("button", {name: "Upgrade"}));
    await userEvent.click(screen.getByRole("button", {name: "Cancel"}));
    expect(upgrading.promise.cancel).toHaveBeenCalled();

    await act(async () => upgrading.reject(Object.assign(new Error("cancelled"), {name: "CancelError"})));

    expect(await screen.findByRole("button", {name: "Upgrade"})).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("says why an upgrade failed", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    vi.mocked(Updates.Upgrade).mockRejectedValue(
      new Error("refused", {
        cause: {message: "permission denied: only an administrator may replace /usr/local/bin/tarhche, so upgrade it with sudo tarhche upgrade in a terminal"},
      }),
    );
    renderDialog();

    await userEvent.click(await screen.findByRole("button", {name: "Upgrade"}));

    expect(await screen.findByText("Tarhche could not be upgraded")).toBeInTheDocument();
    expect(screen.getByText(/so upgrade it with sudo tarhche upgrade in a terminal/)).toBeInTheDocument();
    expect(screen.getByRole("button", {name: "Upgrade"})).toBeInTheDocument();
  });

  it("says why it could not restart, and offers it again", async () => {
    vi.mocked(Updates.Check).mockResolvedValue(found() as never);
    vi.mocked(Updates.Upgrade).mockResolvedValue({version: "0.3.0", path: "/usr/local/bin/tarhche"} as never);
    vi.mocked(Updates.Restart).mockRejectedValue(new Error("refused", {cause: {message: "starting tarhche 0.3.0: no such file"}}));
    const onClose = renderDialog();

    await userEvent.click(await screen.findByRole("button", {name: "Upgrade"}));
    await userEvent.click(await screen.findByRole("button", {name: "Restart now"}));

    expect(await screen.findByText("starting tarhche 0.3.0: no such file")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Later"}));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
