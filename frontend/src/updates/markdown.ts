/**
 * The Markdown release notes are written in, as far as they are shown: a
 * run of a line is text, bold, code or a link to a web page.
 */
export type Inline =
  | {kind: "text"; text: string}
  | {kind: "strong"; children: Inline[]}
  | {kind: "code"; text: string}
  | {kind: "link"; href: string; children: Inline[]};

/** A block of release notes: a heading, a list, a paragraph, code or a rule. */
export type Block =
  | {kind: "heading"; level: number; children: Inline[]}
  | {kind: "list"; ordered: boolean; items: Inline[][]}
  | {kind: "paragraph"; children: Inline[]}
  | {kind: "code"; text: string}
  | {kind: "rule"};

const FENCE = /^\s{0,3}(`{3,}|~{3,})/;
const HEADING = /^\s{0,3}(#{1,6})\s+(.*?)(?:\s+#+)?\s*$/;
const RULE = /^\s{0,3}([-*_])(?:\s*\1){2,}\s*$/;
const BULLET = /^\s*[-*+]\s+(.*)$/;
const NUMBERED = /^\s*\d{1,9}[.)]\s+(.*)$/;

/**
 * The runs of a line, in the order they are written: `code`, [a link](url),
 * <url>, a bare url, **bold** or __bold__, and a character a backslash
 * escapes.
 */
const INLINE =
  /`([^`]+)`|\[([^\]]+)\]\(([^)\s]+)\)|<(https?:\/\/[^>\s]+)>|(https?:\/\/[^\s<]*[^\s<.,:;!?"'*_)\]])|\*\*(.+?)\*\*|__(.+?)__|\\([!-/:-@[-`{-~])/g;

/** A list as it is read, item by item. */
type ListDraft = {ordered: boolean; items: string[]};

/**
 * The blocks Markdown source is made of, as far as release notes are
 * shown: headings, lists (an item's lines that follow it are its own),
 * paragraphs, fenced code and rules. Anything else, HTML among it, is text.
 */
export function parseMarkdown(source: string): Block[] {
  const blocks: Block[] = [];
  const lines = source.replace(/\r\n?/g, "\n").split("\n");
  let paragraph: string[] = [];
  let list: ListDraft | null = null;

  const close = () => {
    if (paragraph.length > 0) {
      blocks.push({kind: "paragraph", children: parseInline(paragraph.join(" "))});
      paragraph = [];
    }

    if (list !== null) {
      blocks.push({kind: "list", ordered: list.ordered, items: list.items.map((item) => parseInline(item))});
      list = null;
    }
  };

  for (let at = 0; at < lines.length; at++) {
    const line = lines[at];

    const fence = FENCE.exec(line);
    if (fence) {
      close();

      const code: string[] = [];
      for (at++; at < lines.length && !lines[at].trimStart().startsWith(fence[1]); at++) {
        code.push(lines[at]);
      }

      blocks.push({kind: "code", text: code.join("\n")});
      continue;
    }

    if (line.trim() === "") {
      close();
      continue;
    }

    const heading = HEADING.exec(line);
    if (heading) {
      close();
      blocks.push({kind: "heading", level: heading[1].length, children: parseInline(heading[2])});
      continue;
    }

    if (RULE.test(line)) {
      close();
      blocks.push({kind: "rule"});
      continue;
    }

    const bullet = BULLET.exec(line);
    const numbered = bullet ? null : NUMBERED.exec(line);
    const item = bullet ?? numbered;
    if (item) {
      const ordered = numbered !== null;
      if (paragraph.length > 0 || (list !== null && list.ordered !== ordered)) {
        close();
      }

      list ??= {ordered, items: []};
      list.items.push(item[1].trim());
      continue;
    }

    if (list !== null) {
      list.items[list.items.length - 1] += ` ${line.trim()}`;
      continue;
    }

    paragraph.push(line.trim());
  }

  close();

  return blocks;
}

/**
 * The runs a line of Markdown is made of. A link is only ever to a web page
 * (http or https): one to anything else is its text alone. Inside a link,
 * linking is off, so that no link holds another.
 */
export function parseInline(text: string, linking = true): Inline[] {
  const runs: Inline[] = [];
  let at = 0;

  for (const match of text.matchAll(INLINE)) {
    const [whole, code, label, href, angled, bare, starred, underlined, escaped] = match;
    if (match.index > at) {
      addText(runs, text.slice(at, match.index));
    }

    if (code !== undefined) {
      runs.push({kind: "code", text: code});
    } else if (label !== undefined) {
      if (linking && isWebPage(href)) {
        runs.push({kind: "link", href, children: parseInline(label, false)});
      } else {
        for (const run of parseInline(label, false)) {
          addRun(runs, run);
        }
      }
    } else if (angled !== undefined || bare !== undefined) {
      const url = angled ?? bare;
      if (linking) {
        runs.push({kind: "link", href: url, children: [{kind: "text", text: url}]});
      } else {
        addText(runs, whole);
      }
    } else if (starred !== undefined || underlined !== undefined) {
      runs.push({kind: "strong", children: parseInline(starred ?? underlined, linking)});
    } else {
      addText(runs, escaped);
    }

    at = match.index + whole.length;
  }

  if (at < text.length) {
    addText(runs, text.slice(at));
  }

  return runs;
}

/** Whether a link points at a web page, the only thing a link is followed to. */
function isWebPage(href: string): boolean {
  try {
    const url = new URL(href);

    return url.protocol === "https:" || url.protocol === "http:";
  } catch {
    return false;
  }
}

/** Adds a run, text joining the text before it. */
function addRun(runs: Inline[], run: Inline) {
  if (run.kind === "text") {
    addText(runs, run.text);
  } else {
    runs.push(run);
  }
}

/** Adds text, joining the text before it. */
function addText(runs: Inline[], text: string) {
  const last = runs[runs.length - 1];
  if (last?.kind === "text") {
    last.text += text;
  } else {
    runs.push({kind: "text", text});
  }
}
