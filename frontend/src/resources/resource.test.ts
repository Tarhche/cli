import {describe, expect, it} from "vitest";
import type {ResourceDefinition} from "../api/services";
import {actionsFor, audienceShown, audiencesOf, filtersFor, nameOf, pretty, routesFor, said} from "./resource";

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

  it("names a row by its key, or its first cell", () => {
    expect(nameOf(["c-1", "en"], ["x"])).toBe("c-1/en");
    expect(nameOf([], ["first"])).toBe("first");
    expect(nameOf([], [])).toBe("");
  });

  it("writes JSON to be read, and leaves out filters that say nothing", () => {
    expect(pretty({a: 1})).toBe('{\n  "a": 1\n}');
    expect(pretty(null)).toBe("{}");
    expect(said({kind: "read", ip: " ", user_uuid: ""})).toEqual({kind: "read"});
  });
});
