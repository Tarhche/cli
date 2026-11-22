import {describe, expect, it} from "vitest";
import type {ResourceDefinition} from "../api/services";
import type {Row} from "../api/services";
import {actionsFor, audienceShown, audiencesOf, filtersFor, linksFor, nameOf, routesFor, rowActions, rowLinks, said} from "./resource";

const definition = {
  name: "audit-log",
  audiences: ["mine", "everyone", "nobody"],
  mine: {list: true, get: false, create: false, update: false, delete: false},
  everyone: {list: true, get: true, create: false, update: false, delete: true},
  actions: [
    {name: "activity", description: "everybody's activity", mine: false, keyed: false, allowed: true},
    {name: "activity", description: "one's own activity", mine: true, keyed: false, allowed: true},
    {name: "finish", description: "", mine: true, keyed: true, allowed: true},
    {name: "read", description: "", mine: false, keyed: true, allowed: true},
    {name: "archive", description: "", mine: false, keyed: true, allowed: false},
  ],
  filters: [
    {name: "kind", description: "", values: ["read"], moment: false, audiences: []},
    {name: "user_uuid", description: "", values: [], moment: false, audiences: ["everyone"]},
  ],
} as unknown as ResourceDefinition;

describe("resources", () => {
  it("lists the audiences it knows", () => {
    expect(audiencesOf(definition)).toEqual(["mine", "everyone"]);
    expect(audiencesOf({...definition, audiences: null} as unknown as ResourceDefinition)).toEqual([]);
  });

  it("asks an audience what it can be asked", () => {
    expect(routesFor(definition, "everyone").delete).toBe(true);
    expect(routesFor(definition, "mine").delete).toBe(false);
  });

  it("offers an action of each name, the audience's own first", () => {
    expect(actionsFor(definition, "mine", false).map((a) => a.description)).toEqual(["one's own activity"]);
    expect(actionsFor(definition, "everyone", false).map((a) => a.description)).toEqual(["everybody's activity"]);
    expect(actionsFor(definition, "everyone", true).map((a) => a.name)).toEqual(["finish", "read"]);
    expect(actionsFor({...definition, actions: null} as unknown as ResourceDefinition, "mine", true)).toEqual([]);
  });

  it("offers no action the session may not ask, though the other audience's of its name it may", () => {
    const disallowed = {
      ...definition,
      actions: [
        {name: "activity", description: "everybody's activity", mine: false, keyed: false, allowed: true},
        {name: "activity", description: "one's own activity", mine: true, keyed: false, allowed: false},
      ],
    } as unknown as ResourceDefinition;

    expect(actionsFor(disallowed, "mine", false)).toEqual([]);
    expect(actionsFor(disallowed, "everyone", false).map((a) => a.description)).toEqual(["everybody's activity"]);
    expect(actionsFor(definition, "everyone", true).map((a) => a.name)).not.toContain("archive");
  });

  it("shows the audience chosen while it may still be listed, else the first that may", () => {
    expect(audienceShown(["mine", "everyone"], "everyone")).toBe("everyone");
    expect(audienceShown(["mine"], "everyone")).toBe("mine");
    expect(audienceShown([], "everyone")).toBe("everyone");
  });

  it("offers the filters an audience's listing takes", () => {
    expect(filtersFor(definition, "mine").map((f) => f.name)).toEqual(["kind"]);
    expect(filtersFor(definition, "everyone").map((f) => f.name)).toEqual(["kind", "user_uuid"]);
    expect(filtersFor({...definition, filters: null} as unknown as ResourceDefinition, "mine")).toEqual([]);
  });

  it("names a row by the first thing it says, or its key", () => {
    expect(nameOf(["c-1", "en"], ["", "Hello"])).toBe("Hello");
    expect(nameOf(["c-1", "en"], [" "])).toBe("c-1/en");
    expect(nameOf([], [])).toBe("");
  });

  it("leaves out filters that say nothing", () => {
    expect(said({kind: "read", ip: " ", user_uuid: ""})).toEqual({kind: "read"});
  });

  it("offers the links of an audience, each a row has somewhere to go for", () => {
    const linked = {
      ...definition,
      links: [
        {name: "View", kind: "view", resource: "", filter: "", audiences: ["mine", "everyone"]},
        {name: "Edit", kind: "edit", resource: "", filter: "", audiences: ["everyone"]},
        {name: "Attempts", kind: "list", resource: "exam-attempt", filter: "exam_uuid", audiences: []},
      ],
    } as unknown as ResourceDefinition;
    const row = {key: ["x"], cells: [], links: {Edit: "https://web.example/edit", Attempts: "x"}, actions: ["read"]} as unknown as Row;

    expect(linksFor(linked, "mine").map((l) => l.name)).toEqual(["View"]);
    expect(linksFor(linked, "everyone").map((l) => l.name)).toEqual(["View", "Edit"]);
    expect(linksFor({...definition, links: null} as unknown as ResourceDefinition, "mine")).toEqual([]);
    expect(rowLinks(linksFor(linked, "everyone"), row).map(({link, to}) => [link.name, to])).toEqual([["Edit", "https://web.example/edit"]]);
    expect(rowLinks(linksFor(linked, "everyone"), {...row, links: null} as unknown as Row)).toEqual([]);
  });

  it("offers a row the actions that hold for it", () => {
    const actions = actionsFor(definition, "everyone", true);

    expect(rowActions(actions, {actions: ["read"]} as unknown as Row).map((a) => a.name)).toEqual(["read"]);
    expect(rowActions(actions, {actions: null} as unknown as Row)).toEqual([]);
  });
});
