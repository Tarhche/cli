import {describe, expect, it} from "vitest";
import {parseInline, parseMarkdown} from "./markdown";

describe("parseMarkdown", () => {
  it("reads GoReleaser's changelog", () => {
    const notes = [
      "## Changelog",
      "### Features",
      "* 1a2b3c4 Feat: upgrades itself by @sarah in https://github.com/Tarhche/cli/pull/2",
      "* 5d6e7f8 Feat: **bold** and `code`",
      "",
      "**Full Changelog**: https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0",
    ].join("\n");

    expect(parseMarkdown(notes)).toEqual([
      {kind: "heading", level: 2, children: [{kind: "text", text: "Changelog"}]},
      {kind: "heading", level: 3, children: [{kind: "text", text: "Features"}]},
      {
        kind: "list",
        ordered: false,
        items: [
          [
            {kind: "text", text: "1a2b3c4 Feat: upgrades itself by @sarah in "},
            {kind: "link", href: "https://github.com/Tarhche/cli/pull/2", children: [{kind: "text", text: "https://github.com/Tarhche/cli/pull/2"}]},
          ],
          [
            {kind: "text", text: "5d6e7f8 Feat: "},
            {kind: "strong", children: [{kind: "text", text: "bold"}]},
            {kind: "text", text: " and "},
            {kind: "code", text: "code"},
          ],
        ],
      },
      {
        kind: "paragraph",
        children: [
          {kind: "strong", children: [{kind: "text", text: "Full Changelog"}]},
          {kind: "text", text: ": "},
          {
            kind: "link",
            href: "https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0",
            children: [{kind: "text", text: "https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0"}],
          },
        ],
      },
    ]);
  });

  it("joins the lines of a paragraph, which a blank line ends", () => {
    expect(parseMarkdown("one\r\ntwo\n\nthree")).toEqual([
      {kind: "paragraph", children: [{kind: "text", text: "one two"}]},
      {kind: "paragraph", children: [{kind: "text", text: "three"}]},
    ]);
  });

  it("reads numbered lists, and the lines after an item as its own", () => {
    expect(parseMarkdown("1. first\n   and more\n2) second\n- a bullet")).toEqual([
      {kind: "list", ordered: true, items: [[{kind: "text", text: "first and more"}], [{kind: "text", text: "second"}]]},
      {kind: "list", ordered: false, items: [[{kind: "text", text: "a bullet"}]]},
    ]);
  });

  it("keeps fenced code as it is written, reading nothing in it", () => {
    expect(parseMarkdown("```sh\n**tarhche** upgrade\n```\nafter")).toEqual([
      {kind: "code", text: "**tarhche** upgrade"},
      {kind: "paragraph", children: [{kind: "text", text: "after"}]},
    ]);
  });

  it("reads rules, and headings with their closing hashes", () => {
    expect(parseMarkdown("# Title ##\n* * *\n---")).toEqual([
      {kind: "heading", level: 1, children: [{kind: "text", text: "Title"}]},
      {kind: "rule"},
      {kind: "rule"},
    ]);
  });

  it("takes HTML as text", () => {
    expect(parseMarkdown('<img src=x onerror="alert(1)"> <b>bold</b>')).toEqual([
      {kind: "paragraph", children: [{kind: "text", text: '<img src=x onerror="alert(1)"> <b>bold</b>'}]},
    ]);
  });

  it("is nothing when there is nothing to read", () => {
    expect(parseMarkdown("  \n\n")).toEqual([]);
  });
});

describe("parseInline", () => {
  it("links to web pages", () => {
    expect(parseInline("[the release](https://github.com/Tarhche/cli/releases/tag/v0.2.0) and <http://example.com/a>")).toEqual([
      {kind: "link", href: "https://github.com/Tarhche/cli/releases/tag/v0.2.0", children: [{kind: "text", text: "the release"}]},
      {kind: "text", text: " and "},
      {kind: "link", href: "http://example.com/a", children: [{kind: "text", text: "http://example.com/a"}]},
    ]);
  });

  it("links to nothing but web pages: a link to anything else is its text", () => {
    expect(parseInline("[click](javascript:alert(1)) [mail](mailto:sarah@example.com)")).toEqual([{kind: "text", text: "click) mail"}]);
  });

  it("holds no link inside a link", () => {
    expect(parseInline("[see https://example.com](https://example.com/a)")).toEqual([
      {kind: "link", href: "https://example.com/a", children: [{kind: "text", text: "see https://example.com"}]},
    ]);
  });

  it("ends a bare link before the punctuation after it", () => {
    expect(parseInline("see https://example.com/a.")).toEqual([
      {kind: "text", text: "see "},
      {kind: "link", href: "https://example.com/a", children: [{kind: "text", text: "https://example.com/a"}]},
      {kind: "text", text: "."},
    ]);
  });

  it("reads bold either way, and what a backslash escapes as text", () => {
    expect(parseInline("__strong__ \\*not bold\\*")).toEqual([
      {kind: "strong", children: [{kind: "text", text: "strong"}]},
      {kind: "text", text: " *not bold*"},
    ]);
  });
});
