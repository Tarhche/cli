import {describe, expect, it} from "vitest";
import type {ResourceDefinition} from "../api/services";
import {HOME, pageOf, popped, pushed, sectionsOf} from "./navigation";

function definition(name: string, section: string, audiences: string[]): ResourceDefinition {
  return {name, section, audiences, title: name} as unknown as ResourceDefinition;
}

describe("sectionsOf", () => {
  it("groups what is listed by section, in the order the sections are first named", () => {
    const sections = sectionsOf([
      definition("articles", "Content", ["mine", "everyone"]),
      definition("users", "Administration", ["everyone"]),
      definition("comments", "Content", ["mine"]),
      definition("hidden", "Content", []),
      definition("loose", "", ["everyone"]),
    ]);

    expect(sections.map((s) => [s.title, s.definitions.map((d) => d.name)])).toEqual([
      ["Content", ["articles", "comments"]],
      ["Administration", ["users"]],
      ["Other", ["loose"]],
    ]);
  });

  it("is nothing for no definitions", () => {
    expect(sectionsOf([])).toEqual([]);
  });
});

describe("pageOf", () => {
  it("is the listing a detail is under", () => {
    expect(pageOf({page: "vm", uuid: "v"})).toBe("vms");
    expect(pageOf({page: "create-vm"})).toBe("vms");
    expect(pageOf({page: "container", vm: "v", id: "c"})).toBe("containers");
    expect(pageOf({page: "run-container"})).toBe("containers");
    expect(pageOf({page: "deploy-stack"})).toBe("stacks");
    expect(pageOf({page: "resource", name: "articles"})).toBe("resource:articles");
    expect(pageOf({page: "images"})).toBe("images");
  });
});

describe("history", () => {
  it("goes forward, not twice to the same view, and back", () => {
    const forward = pushed(pushed([HOME], {page: "vms"}), {page: "vm", uuid: "v"});

    expect(forward).toEqual([HOME, {page: "vms"}, {page: "vm", uuid: "v"}]);
    expect(pushed(forward, {page: "vm", uuid: "v"})).toBe(forward);
    expect(popped(forward)).toEqual([HOME, {page: "vms"}]);
  });

  it("goes back home when there is nothing before", () => {
    expect(popped([{page: "vms"}])).toEqual([HOME]);
    expect(popped([])).toEqual([HOME]);
  });
});
