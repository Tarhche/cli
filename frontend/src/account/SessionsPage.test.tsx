import {modals} from "@mantine/modals";
import {notifications} from "@mantine/notifications";
import {act, screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach, beforeEach, describe, expect, it, vi} from "vitest";
import {type Listing, Resources} from "../api/services";
import {render} from "../test/render";
import type {LoginSession} from "./sessions";
import {SessionsPage} from "./SessionsPage";

vi.mock("../api/services", () => ({
  Resources: {List: vi.fn(), Delete: vi.fn(), Action: vi.fn()},
}));

const minutesAgo = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString();

/** The session the app is signed in with: the CLI's, handed to it as an application. */
const desktop: LoginSession = {
  uuid: "s-desktop",
  current: true,
  device: {user_agent: "tarhche/0.2.0", os: "macOS", type: "desktop"},
  client: {id: "c-1", name: "Tarhche CLI"},
  ip: "203.0.113.7",
  created_at: minutesAgo(600),
  last_activity: {at: minutesAgo(0), ip: "203.0.113.7"},
  expires_at: minutesAgo(-60 * 24 * 30),
};

/** A browser signed in to on a phone. */
const phone: LoginSession = {
  uuid: "s-phone",
  current: false,
  device: {
    user_agent: "Mozilla/5.0 (iPhone)",
    fingerprint: "fp-7f3a",
    browser: "Safari",
    browser_version: "18.1.2",
    os: "iOS",
    os_version: "18.1",
    type: "mobile",
    model: "iPhone",
    language: "en-GB",
    timezone: "Europe/Berlin",
    screen: "390x844",
  },
  ip: "198.51.100.4",
  created_at: minutesAgo(60 * 24 * 3),
  last_activity: {at: minutesAgo(5), ip: "198.51.100.9"},
  expires_at: minutesAgo(-60 * 24 * 27),
};

/** A shadow session, seen as somebody else. */
const shadow: LoginSession = {
  uuid: "s-shadow",
  current: false,
  device: {browser: "Firefox", browser_version: "128.0", os: "Linux", type: "desktop"},
  impersonated_user: {uuid: "u-2", name: "Sam Rivera", username: "sam"},
  created_at: minutesAgo(120),
  last_activity: {at: minutesAgo(120)},
  expires_at: minutesAgo(-60 * 24 * 29),
};

function listing(...sessions: LoginSession[]): Listing {
  return {
    columns: [],
    rows: sessions.map((session) => ({key: [session.uuid], cells: [], item: session})),
    total_pages: 0,
    current_page: 0,
    next: "",
  } as unknown as Listing;
}

describe("SessionsPage", () => {
  beforeEach(() => {
    vi.mocked(Resources.List).mockReset().mockResolvedValue(listing(desktop, phone, shadow) as never);
    vi.mocked(Resources.Delete).mockReset().mockResolvedValue(undefined as never);
    vi.mocked(Resources.Action).mockReset().mockResolvedValue(null as never);
  });

  afterEach(() => {
    vi.useRealTimers();
    act(() => {
      modals.closeAll();
      notifications.clean();
    });
  });

  it("lists this device first, then every other session, as the web does", async () => {
    render(<SessionsPage />);

    expect(await screen.findByRole("button", {name: /Tarhche CLI/})).toBeInTheDocument();
    expect(Resources.List).toHaveBeenCalledWith("session", false, {page: 1, cursor: "", filters: {}});

    const headings = screen.getAllByRole("heading", {level: 4}).map((heading) => heading.textContent);
    expect(headings).toEqual(["This device", "Other sessions"]);

    const current = screen.getByRole("button", {name: /Tarhche CLI/});
    expect(within(current).getByText("This device")).toBeInTheDocument();
    expect(within(current).getByText("macOS · Desktop")).toBeInTheDocument();
    expect(within(current).getByText("203.0.113.7 · Active now")).toBeInTheDocument();

    const other = screen.getByRole("button", {name: /Safari 18/});
    expect(within(other).getByText("iPhone · iOS 18.1 · Phone")).toBeInTheDocument();
    expect(within(other).getByText("198.51.100.9 · Last active 5 minutes ago")).toBeInTheDocument();

    const seen = screen.getByRole("button", {name: /Firefox 128/});
    expect(within(seen).getByText("Seen as Sam Rivera")).toBeInTheDocument();
    expect(within(seen).getByText("Last active 2 hours ago")).toBeInTheDocument();

    expect(screen.queryByText("s-phone")).not.toBeInTheDocument();
  });

  it("opens a session to show what it was signed in with and where it was used from", async () => {
    render(<SessionsPage />);

    await userEvent.click(await screen.findByRole("button", {name: /Safari 18/}));

    const details = await screen.findByRole("region", {name: /Safari 18/});
    expect(within(details).getByText("Safari 18.1.2")).toBeVisible();
    expect(within(details).getByText("Operating system")).toBeInTheDocument();
    expect(within(details).getByText("Signed in from")).toBeInTheDocument();
    expect(within(details).getByText("198.51.100.4")).toBeInTheDocument();
    expect(within(details).getByText("Europe/Berlin")).toBeInTheDocument();
    expect(within(details).getByText("fp-7f3a")).toBeInTheDocument();
    expect(within(details).getByText("Mozilla/5.0 (iPhone)")).toBeInTheDocument();
    expect(within(details).getByRole("button", {name: "Terminate session"})).toBeInTheDocument();
    expect(screen.queryByText("s-phone")).not.toBeInTheDocument();
  });

  it("never offers to terminate this device", async () => {
    render(<SessionsPage />);

    await userEvent.click(await screen.findByRole("button", {name: /Tarhche CLI/}));

    const details = await screen.findByRole("region", {name: /Tarhche CLI/});
    expect(within(details).getByText("tarhche/0.2.0")).toBeVisible();
    expect(within(details).queryByRole("button", {name: "Terminate session"})).not.toBeInTheDocument();
  });

  it("terminates another session once that is confirmed, and reads them again", async () => {
    render(<SessionsPage />);
    await userEvent.click(await screen.findByRole("button", {name: /Safari 18/}));

    await userEvent.click(await screen.findByRole("button", {name: "Terminate session"}));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Terminate Safari 18 (iPhone · iOS 18.1 · Phone)? Whatever is signed in with it is signed out at its next request.")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", {name: "Terminate session"}));

    await waitFor(() => expect(Resources.Delete).toHaveBeenCalledWith("session", false, ["s-phone"]));
    expect(await screen.findByText("The session was terminated")).toBeInTheDocument();
    await waitFor(() => expect(Resources.List).toHaveBeenCalledTimes(2));
  });

  it("terminates nothing when it is not confirmed", async () => {
    render(<SessionsPage />);
    await userEvent.click(await screen.findByRole("button", {name: /Safari 18/}));

    await userEvent.click(await screen.findByRole("button", {name: "Terminate session"}));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", {name: "Cancel"}));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(Resources.Delete).not.toHaveBeenCalled();
  });

  it("takes a session that is gone already as terminated", async () => {
    vi.mocked(Resources.Delete).mockRejectedValue(new Error("x", {cause: {message: "not found", status: 404}}));
    render(<SessionsPage />);
    await userEvent.click(await screen.findByRole("button", {name: /Safari 18/}));

    await userEvent.click(await screen.findByRole("button", {name: "Terminate session"}));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", {name: "Terminate session"}));

    expect(await screen.findByText("The session was terminated")).toBeInTheDocument();
  });

  it("says why a session could not be terminated", async () => {
    vi.mocked(Resources.Delete).mockRejectedValue(new Error("x", {cause: {message: "the estate is away", status: 503}}));
    render(<SessionsPage />);
    await userEvent.click(await screen.findByRole("button", {name: /Safari 18/}));

    await userEvent.click(await screen.findByRole("button", {name: "Terminate session"}));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", {name: "Terminate session"}));

    expect(await screen.findByText("the estate is away")).toBeInTheDocument();
    expect(screen.queryByText("The session was terminated")).not.toBeInTheDocument();
  });

  it("terminates every other session once that is confirmed, saying how many", async () => {
    render(<SessionsPage />);
    await screen.findByRole("button", {name: /Tarhche CLI/});

    await userEvent.click(screen.getByRole("button", {name: "Terminate all other sessions"}));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Terminate the 2 other sessions? Only this device stays signed in.")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", {name: "Terminate all other sessions"}));

    await waitFor(() => expect(Resources.Action).toHaveBeenCalledWith("session", "rm-others", false, []));
    expect(await screen.findByText("All other sessions were terminated")).toBeInTheDocument();
    await waitFor(() => expect(Resources.List).toHaveBeenCalledTimes(2));
  });

  it("says when the account is signed in nowhere else, and offers to terminate nothing", async () => {
    vi.mocked(Resources.List).mockResolvedValue(listing(desktop) as never);
    render(<SessionsPage />);

    expect(await screen.findByText("Your account is not signed in anywhere else.")).toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Terminate all other sessions"})).not.toBeInTheDocument();
  });

  it("says why they could not be read, and reads them again when asked", async () => {
    vi.mocked(Resources.List).mockRejectedValueOnce(new Error("x", {cause: {message: "the estate is away", status: 503}}));
    render(<SessionsPage />);

    expect(await screen.findByText("the estate is away")).toBeInTheDocument();
    expect(screen.getByText("Your sessions could not be read")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", {name: "Retry"}));

    expect(await screen.findByRole("button", {name: /Tarhche CLI/})).toBeInTheDocument();
    expect(screen.queryByText("the estate is away")).not.toBeInTheDocument();
  });

  it("reads them again every minute, so that their activity is fresh", async () => {
    vi.useFakeTimers();
    render(<SessionsPage />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(Resources.List).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });

    expect(Resources.List).toHaveBeenCalledTimes(2);
  });
});
