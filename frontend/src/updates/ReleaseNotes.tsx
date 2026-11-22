import {Anchor, Code, Divider, List, Stack, Text, Title, type TitleOrder} from "@mantine/core";
import {Fragment, useMemo} from "react";
import {openInBrowser} from "../api/browser";
import {type Block, type Inline, parseMarkdown} from "./markdown";

/**
 * What a release says of itself, its Markdown shown as Mantine's text: none
 * of it becomes HTML, and its links open in the system's browser.
 */
export function ReleaseNotes({notes}: {notes: string}) {
  const blocks = useMemo(() => parseMarkdown(notes), [notes]);

  if (blocks.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        The release says nothing of itself.
      </Text>
    );
  }

  return (
    <Stack gap="xs">
      {blocks.map((block, at) => (
        <BlockOf key={at} block={block} />
      ))}
    </Stack>
  );
}

/** The order a heading of a level is shown at, below the dialog's title. */
function orderOf(level: number): TitleOrder {
  return Math.min(level + 3, 6) as TitleOrder;
}

/** A block of release notes. */
function BlockOf({block}: {block: Block}) {
  switch (block.kind) {
    case "heading":
      return (
        <Title order={orderOf(block.level)}>
          <Runs runs={block.children} />
        </Title>
      );
    case "list":
      return (
        <List size="sm" type={block.ordered ? "ordered" : "unordered"} spacing={4}>
          {block.items.map((item, at) => (
            <List.Item key={at}>
              <Runs runs={item} />
            </List.Item>
          ))}
        </List>
      );
    case "code":
      return <Code block>{block.text}</Code>;
    case "rule":
      return <Divider />;
    default:
      return (
        <Text size="sm">
          <Runs runs={block.children} />
        </Text>
      );
  }
}

/** The runs of a line of release notes. */
function Runs({runs}: {runs: Inline[]}) {
  return runs.map((run, at) => <Fragment key={at}>{runOf(run)}</Fragment>);
}

/** A run of a line of release notes. */
function runOf(run: Inline) {
  switch (run.kind) {
    case "strong":
      return (
        <Text span inherit fw={700}>
          <Runs runs={run.children} />
        </Text>
      );
    case "code":
      return <Code>{run.text}</Code>;
    case "link":
      return (
        <Anchor href={run.href} inherit onClick={(event) => openInBrowser(event, run.href)}>
          <Runs runs={run.children} />
        </Anchor>
      );
    default:
      return run.text;
  }
}
