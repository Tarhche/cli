import {screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {beforeEach, describe, expect, it, vi} from "vitest";
import {type Listing, type ResourceDefinition, Resources} from "../api/services";
import {render} from "../test/render";
import {ResourcePage} from "./ResourcePage";

vi.mock("../api/services", () => ({
  Resources: {List: vi.fn(), Get: vi.fn(), Create: vi.fn(), Update: vi.fn(), Delete: vi.fn(), Action: vi.fn()},
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
  actions: [{name: "publish", description: "Publish it", mine: false, keyed: true}],
  filters: [{name: "language_code", description: "only a language", values: ["en", "fa"], moment: false, audiences: []}],
  cursor: false,
  columns: ["ID", "LANGUAGE", "TITLE"],
} as unknown as ResourceDefinition;

const listing = {
  columns: ["ID", "LANGUAGE", "TITLE"],
  rows: [{key: ["c-1", "en"], cells: ["c-1", "en", "Hello"], item: {title: "Hello"}}],
  total_pages: 1,
  current_page: 1,
  next: "",
} as unknown as Listing;

describe("ResourcePage", () => {
  beforeEach(() => {
    vi.mocked(Resources.List).mockReset().mockResolvedValue(listing as never);
  });

  it("lists one's own as the definition's table", async () => {
    render(<ResourcePage definition={articles} />);

    expect(await screen.findByText("Hello")).toBeInTheDocument();
    expect(screen.getByRole("heading", {name: "Articles"})).toBeInTheDocument();
    expect(screen.getByRole("columnheader", {name: "TITLE"})).toBeInTheDocument();
    expect(Resources.List).toHaveBeenCalledWith("article", false, {page: 1, cursor: "", filters: {}});
    expect(screen.queryByRole("button", {name: "Create"})).not.toBeInTheDocument();
  });

  it("lists everybody's when asked, and offers to create one", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByText("Everybody's"));

    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", true, {page: 1, cursor: "", filters: {}}));
    expect(screen.getByRole("button", {name: "Create"})).toBeInTheDocument();
  });

  it("narrows the listing by its filters", async () => {
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("combobox", {name: "language_code"}));
    await userEvent.click(await screen.findByRole("option", {name: "fa"}));
    await userEvent.click(screen.getByRole("button", {name: "Filter"}));

    await waitFor(() => expect(Resources.List).toHaveBeenLastCalledWith("article", false, {page: 1, cursor: "", filters: {language_code: "fa"}}));
  });

  it("deletes a row, once that is confirmed", async () => {
    vi.mocked(Resources.Delete).mockResolvedValue(undefined as never);
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("button", {name: "What to do with c-1/en"}));
    await userEvent.click(await screen.findByRole("menuitem", {name: "Delete"}));
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", {name: "Delete"}));

    await waitFor(() => expect(Resources.Delete).toHaveBeenCalledWith("article", false, ["c-1", "en"]));
  });

  it("asks a row an action, and shows what it answered", async () => {
    vi.mocked(Resources.Action).mockResolvedValue({published: true} as never);
    render(<ResourcePage definition={articles} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("button", {name: "What to do with c-1/en"}));
    await userEvent.click(await screen.findByRole("menuitem", {name: "Publish it"}));

    await waitFor(() => expect(Resources.Action).toHaveBeenCalledWith("article", "publish", false, ["c-1", "en"]));
    expect(await screen.findByText(/"published": true/)).toBeInTheDocument();
  });

  it("creates one from the JSON written", async () => {
    vi.mocked(Resources.Create).mockResolvedValue({} as never);
    render(<ResourcePage definition={{...articles, audiences: ["everyone"]} as unknown as ResourceDefinition} />);
    await screen.findByText("Hello");

    await userEvent.click(screen.getByRole("button", {name: "Create"}));
    const editor = await screen.findByRole("textbox", {name: "JSON"});
    await userEvent.clear(editor);
    await userEvent.type(editor, '{{"title":"New"}');
    await userEvent.click(screen.getByRole("button", {name: "Save"}));

    await waitFor(() => expect(Resources.Create).toHaveBeenCalled());
    const [name, everyone, json] = vi.mocked(Resources.Create).mock.calls[0];
    expect([name, everyone, JSON.parse(json)]).toEqual(["article", true, {title: "New"}]);
  });

  it("says why the listing could not be read", async () => {
    vi.mocked(Resources.List).mockRejectedValue(new Error("x", {cause: {message: "forbidden", status: 403}}));
    render(<ResourcePage definition={articles} />);

    expect(await screen.findByText("forbidden")).toBeInTheDocument();
  });
});
