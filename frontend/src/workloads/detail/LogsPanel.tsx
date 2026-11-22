import {Button, Code, Group, ScrollArea, Select, Stack, Switch, Text} from "@mantine/core";
import {IconRefresh} from "@tabler/icons-react";
import {useEffect, useRef, useState} from "react";
import type {LogsQuery} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {FailureAlert} from "../../components/FailureAlert";
import {lineOf} from "../logs";
import type {Logs} from "../types";

/** How often a log that is followed is read again. */
const FOLLOW = 2000;

const TAILS = ["100", "500", "1000", "5000"];

type Props = {
  /** What the log is of, which it is read again for when it changes. */
  id: string;

  /** Reads the log. */
  read: (query: LogsQuery) => Promise<Logs>;

  /** Whether the log is shown, which it is only read while it is. */
  active: boolean;
};

/** What a VM or a container wrote, its last lines, followed or not. */
export function LogsPanel({id, read, active}: Props) {
  const [tail, setTail] = useState("500");
  const [follow, setFollow] = useState(true);
  const viewport = useRef<HTMLDivElement>(null);
  const logs = useQuery(`logs:${id}:${tail}`, () => read({tail: Number(tail)}), {interval: active && follow ? FOLLOW : 0, enabled: active});
  const lines = logs.data?.items ?? [];

  useEffect(() => {
    if (follow) {
      viewport.current?.scrollTo({top: viewport.current.scrollHeight});
    }
  }, [logs.data, follow]);

  return (
    <Stack>
      <Group align="flex-end">
        <Select label="Last lines" data={TAILS} value={tail} onChange={(value) => setTail(value ?? "500")} allowDeselect={false} w={140} />
        <Switch label="Follow" checked={follow} onChange={(event) => setFollow(event.currentTarget.checked)} mb={8} />
        <Button variant="default" leftSection={<IconRefresh size={16} />} onClick={logs.reload}>
          Refresh
        </Button>
      </Group>
      <FailureAlert failure={logs.failure} title="The log could not be read" />
      {logs.data?.truncated && (
        <Text size="xs" c="dimmed">
          More was written than is shown: these are its last lines.
        </Text>
      )}
      <ScrollArea h={480} type="auto" viewportRef={viewport}>
        <Code block style={{whiteSpace: "pre-wrap", wordBreak: "break-all", minHeight: 480}}>
          {lines.length > 0 ? lines.map(lineOf).join("\n") : logs.loading ? "Reading…" : "Nothing was written yet."}
        </Code>
      </ScrollArea>
    </Stack>
  );
}
