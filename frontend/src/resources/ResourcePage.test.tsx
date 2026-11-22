import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {Browser} from "@wailsio/runtime";
import type {ReactElement} from "react";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {type Listing, type ResourceDefinition, Resources} from "../api/services";
import {type Navigation, NavigationContext} from "../shell/navigation";
import {render as renderThemed} from "../test/render";
import {ResourcePage} from "./ResourcePage";

vi.mock("../api/services", () => ({
  Resources: {List: vi.fn(), Delete: vi.fn(), Action: vi.fn()},
}));

const routes = {list: true, get: true, create: true, update: true, delete: true};

const articles = {
  name: "article",
  plural: "articles",
  title: "Articles",
  description: "What is written",
  section: "Content",
  key: ["correlation_uuid", "language_code"],
  audiences: ["mine", "everyone"],
  mine: {...routes, create: false},
  everyone: routes,
  actions: [
    {name: "publish", description: "Publishes it", title: "Publish", confirm: "", mine: false, keyed: true, allowed: true},
    {name: "archive", description: "Archives it", title: "Archive", confirm: "Archive it for good?", mine: false, keyed: true, allowed: true},
    {name: "export", description: "Exports them", title: "Export", confirm: "", mine: false, keyed: false, allowed: false},
  ],
  filters: [{name: "language_code", description: "only a language", values: ["en", "fa"], moment: false, audiences: []}],
  cursor: false,
  columns: ["Title", "Language"],
  new: {name: "New article", url: "https://web.example/dashboard/articles/new"},
  links: [
    {name: "View article", kind: "view", resource: "", filter: "", audiences: ["mine", "everyone"]},
    {name: "Edit article", kind: "edit", resource: "", filter: "", audiences: ["everyone"]},
    {name: "Comments", kind: "list", resource: "comment", filter: "object_uuid", audiences: ["mine", "everyone"]},
  ],
} as unknown as ResourceDefinition;

const listing = {
  columns: ["Title", "Language"],
  rows: [
    {
      key: ["c-1", "en"],
      cells: ["Hello", "en"],
      item: {title: "Hello"},
      links: {"View article": "https://web.example/en/articles/c-1", "Edit article": "https://web.example/dashboard/articles/c-1/en", Comments: "c-1"},
      actions: ["publish", "archive"],
      protected: false,
    },
    {key: ["c-2", "fa"], cells: ["Draft", "fa"], item: {}, links: {}, actions: [], protected: true},
  ],
  total_pages: 1,
  current_page: 1,
  next: "",
} as unknown as Listing;

/** Where a page is, navigating as navigate says. */
function navigationTo(navigate = vi.fn()): Navigation {
  return {view: {page: "resource", name: "article"}, home: {page: "containers"}, navigate, back: vi.fn()};
}

/** Renders ui where navigating is navigate. */
function render(ui: ReactElement, navigate = vi.fn()) {
  return renderThemed(<NavigationContext.Provider value={navigationTo(navigate)}>{ui}</NavigationContext.Provider>);
}

/** Opens the menu of what to do with the row named name. */
async function menuOf(name: string) {
  await userEvent.click(await screen.findByRole("button", {name: `What to do with ${name}`}));
}

describe("ResourcePage", () => {
  beforeEach(() => {
    vi.mocked(Resources.List).mockReset().mockResolvedValue(listing as never);
    vi.mocked(Resources.Action).mockReset().mockResolvedValue(null as never);
    vi.mocked(Browser.OpenURL).mockClear();
  });

  it("lists one's own as the definition's table", async () => {
    render(<ResourcePage definition={articles} />);

    expect(await screen.findByText("Hello")).toBeInTheDocument();
    expect(screen.getByRole("heading", {name: "Articles"})).toBeInTheDocument();
    expect(screen.getByRole("columnheader", {name: "Title"})).toBeInTheDocument();
    expect(Resources.List).toHaveBeenCalledWith("article", false, {page: 1, cursor: "", filters: {}});
  });

  it("opens on the audience and the filters it is asked to", async () => {
    render(<ResourcePage definition={articles} audience="everyone" filters={{language_code: "fa"}} />);

    await waitFor(() => expect(Resources.List).toHaveBeenCalledWith("article", true, {page: 1, cursor: "", filters: {language_code: "fa"}}));
  });

  it("makes a new one on the web dashboard, in the browser", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("button", {name: "New article"}));

    expect(Browser.OpenURL).toHaveBeenCalledWith("https://web.example/dashboard/articles/new");
  });

  it("offers nothing to make where the session may make nothing, nor an action of the resource it may not ask", async () => {
    render(<ResourcePage definition={{...articles, new: {name: "", url: ""}} as unknown as ResourceDefinition} />);
    await screen.findByText("Hello");

    expect(screen.queryByRole("button", {name: "New article"})).not.toBeInTheDocument();
    expect(screen.queryByRole("button", {name: "Export"})).not.toBeInTheDocument();
  });

  it("lists everybody's when asked", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByText("Everybody's"));

    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", true, {page: 1, cursor: "", filters: {}}));
  });

  it("narrows the listing by its filters", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("combobox", {name: "language_code"}));
    await userEvent.click(await screen.findByRole("option", {name: "fa"}));
    await userEvent.click(screen.getByRole("button", {name: "Filter"}));

    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", false, {page: 1, cursor: "", filters: {language_code: "fa"}}));
  });

  it("offers a row the links of its audience it has a page for, its actions and its delete, and nothing to inspect", async () => {
    render(<ResourcePage definition={articles} />);
    await menuOf("Hello");

    const items = (await screen.findAllByRole("menuitem")).map((item) => item.textContent);

    expect(items).toEqual(["View article", "Comments", "Publish", "Archive", "Delete"]);
  });

  it("opens a page of the website in the browser", async () => {
    render(<ResourcePage definition={articles} audience="everyone" />);
    await menuOf("Hello");

    await userEvent.click(await screen.findByRole("menuitem", {name: /Edit article/}));

    expect(Browser.OpenURL).toHaveBeenCalledWith("https://web.example/dashboard/articles/c-1/en");
  });

  it("goes to the listing a link of the app names, narrowed to the row", async () => {
    const navigate = vi.fn();
    render(<ResourcePage definition={articles} />, navigate);
    await menuOf("Hello");

    await userEvent.click(await screen.findByRole("menuitem", {name: "Comments"}));

    expect(navigate).toHaveBeenCalledWith({page: "resource", name: "comment", audience: "everyone", filters: {object_uuid: "c-1"}});
    expect(Browser.OpenURL).not.toHaveBeenCalled();
  });

  it("offers nothing for a row with no link, no action, and that is not deleted", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Draft");

    expect(screen.queryByRole("button", {name: "What to do with Draft"})).not.toBeInTheDocument();
  });

  it("deletes a row, once that is confirmed", async () => {
    vi.mocked(Resources.Delete).mockResolvedValue(undefined as never);
    render(<ResourcePage definition={articles} />);
    await menuOf("Hello");

    await userEvent.click(await screen.findByRole("menuitem", {name: "Delete"}));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", {name: "Delete"}));

    await waitFor(() => expect(Resources.Delete).toHaveBeenCalledWith("article", false, ["c-1", "en"]));
  });

  it("asks a row an action, and says it was done", async () => {
    render(<ResourcePage definition={articles} />);
    await menuOf("Hello");

    await userEvent.click(await screen.findByRole("menuitem", {name: "Publish"}));

    await waitFor(() => expect(Resources.Action).toHaveBeenCalledWith("article", "publish", false, ["c-1", "en"]));
    expect(await screen.findByText("Publish: done")).toBeInTheDocument();
  });

  it("asks first what an action says to, and asks nothing when that is cancelled", async () => {
    render(<ResourcePage definition={articles} />);
    await menuOf("Hello");

    await userEvent.click(await screen.findByRole("menuitem", {name: "Archive"}));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Archive it for good?")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", {name: "Cancel"}));

    expect(Resources.Action).not.toHaveBeenCalled();

    await menuOf("Hello");
    await userEvent.click(await screen.findByRole("menuitem", {name: "Archive"}));
    await userEvent.click(within(await screen.findByRole("dialog")).getByRole("button", {name: "Archive"}));

    await waitFor(() => expect(Resources.Action).toHaveBeenCalledWith("article", "archive", false, ["c-1", "en"]));
  });

  it("offers no action the session may not ask, nor a delete its routes do not allow", async () => {
    const restricted = {
      ...articles,
      mine: {list: true, get: false, create: false, update: false, delete: false},
      actions: [{...articles.actions[0], allowed: false}, articles.actions[1]],
    } as unknown as ResourceDefinition;
    render(<ResourcePage definition={restricted} />);
    await menuOf("Hello");

    const items = (await screen.findAllByRole("menuitem")).map((item) => item.textContent);

    expect(items).toEqual(["View article", "Comments", "Archive"]);
  });

  it("offers no choice of whose when the session may list one audience alone", async () => {
    render(<ResourcePage definition={{...articles, audiences: ["mine"]} as unknown as ResourceDefinition} />);
    await screen.findByText("Hello");

    expect(screen.queryByText("Everybody's")).not.toBeInTheDocument();
  });

  it("lists the audience it may still list once the one shown may not be listed any more", async () => {
    const {rerender} = render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");
    await userEvent.click(screen.getByText("Everybody's"));
    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", true, {page: 1, cursor: "", filters: {}}));

    rerender(
      <NavigationContext.Provider value={navigationTo()}>
        <ResourcePage definition={{...articles, audiences: ["mine"]} as unknown as ResourceDefinition} />
      </NavigationContext.Provider>,
    );

    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", false, {page: 1, cursor: "", filters: {}}));
  });

  it("says why the listing could not be read", async () => {
    vi.mocked(Resources.List).mockRejectedValue(new Error("x", {cause: {message: "forbidden", status: 403}}));
    render(<ResourcePage definition={articles} />);

    expect(await screen.findByText("forbidden")).toBeInTheDocument();
  });
});
