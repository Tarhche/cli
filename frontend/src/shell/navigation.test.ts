import {describe, expect, it} from "vitest";
import type {ResourceDefinition} from "../api/services";
import {abilities, access} from "../test/access";
import {allows, HOME, homeOf, pageOf, popped, pushed, sectionsOf, workloadPagesOf} from "./navigation";

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
    expect(pageOf({page: "stack", uuid: "s"})).toBe("stacks");
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
    expect(popped([{page: "vm", uuid: "v"}], {page: "stacks"})).toEqual([{page: "stacks"}]);
  });
});

describe("workloadPagesOf", () => {
  it("offers a page of the workload only when its listing is allowed", () => {
    const pages = workloadPagesOf(abilities({vms: {list: true}, stacks: {list: true}, images: {create: true}}));

    expect(pages.map((p) => p.page)).toEqual(["vms", "stacks"]);
  });

  it("offers port forwards only when something may be forwarded", () => {
    expect(workloadPagesOf(abilities({vms: {list: true}})).map((p) => p.page)).toEqual(["vms"]);
    expect(workloadPagesOf(abilities({vms: {list: true, forward: true}})).map((p) => p.page)).toEqual(["vms", "forwards"]);
    expect(workloadPagesOf(abilities({containers: {list: true, exec: true}})).map((p) => p.page)).toEqual(["containers", "forwards"]);
  });

  it("offers nothing when nothing is allowed", () => {
    expect(workloadPagesOf(abilities())).toEqual([]);
  });
});

describe("allows", () => {
  it("lets a session see a detail it may read and a form it may send", () => {
    const may = access({abilities: abilities({vms: {show: true}, containers: {create: true}, stacks: {show: true}})});

    expect(allows(may, {page: "vm", uuid: "v"})).toBe(true);
    expect(allows(may, {page: "create-vm"})).toBe(false);
    expect(allows(may, {page: "container", vm: "v", id: "c"})).toBe(false);
    expect(allows(may, {page: "run-container"})).toBe(true);
    expect(allows(may, {page: "stack", uuid: "s"})).toBe(true);
    expect(allows(may, {page: "deploy-stack"})).toBe(false);
    expect(allows(may, {page: "vms"})).toBe(false);
  });

  it("lets a session see a resource it may list", () => {
    const may = access({definitions: [definition("articles", "Content", ["mine"]), definition("hidden", "Content", [])]});

    expect(allows(may, {page: "resource", name: "articles"})).toBe(true);
    expect(allows(may, {page: "resource", name: "hidden"})).toBe(false);
    expect(allows(may, {page: "resource", name: "users"})).toBe(false);
  });
});

describe("homeOf", () => {
  it("is the first page offered: the workload's, then a resource's", () => {
    expect(homeOf(access())).toEqual({page: "containers"});
    expect(homeOf(access({abilities: abilities({snapshots: {list: true}})}))).toEqual({page: "snapshots"});
    expect(homeOf(access({abilities: abilities(), definitions: [definition("articles", "Content", ["everyone"])]}))).toEqual({page: "resource", name: "articles"});
  });

  it("is nowhere when nothing is offered", () => {
    expect(homeOf(access({abilities: abilities()}))).toBeNull();
  });
});
