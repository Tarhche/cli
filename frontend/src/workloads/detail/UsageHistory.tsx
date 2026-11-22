import {Paper, SimpleGrid, Text} from "@mantine/core";
import {Sparkline} from "../../components/Sparkline";
import {bytes} from "../format";
import type {Usage} from "../types";

/** How busy something was over a span, and how much memory it held. */
export function UsageHistory({usage}: {usage: Usage | undefined}) {
  const points = usage?.points ?? [];
  if (points.length === 0) {
    return (
      <Text size="sm" c="dimmed">
        Nothing was sampled over the last hour.
      </Text>
    );
  }

  const memoryLimit = Math.max(...points.map((p) => p.memory_limit));
  const peakMemory = Math.max(...points.map((p) => p.memory_used));
  const peakCPU = Math.max(...points.map((p) => p.cpu_percent));

  return (
    <SimpleGrid cols={{base: 1, sm: 2}}>
      <Paper withBorder p="sm">
        <Text size="sm" fw={500}>
          CPU, at most {peakCPU.toFixed(1)}%
        </Text>
        <Sparkline label="CPU over the last hour" values={points.map((p) => p.cpu_percent)} max={100} color="indigo" />
      </Paper>
      <Paper withBorder p="sm">
        <Text size="sm" fw={500}>
          Memory, at most {bytes(peakMemory)}
        </Text>
        <Sparkline label="Memory over the last hour" values={points.map((p) => p.memory_used)} max={memoryLimit} color="teal" />
      </Paper>
    </SimpleGrid>
  );
}
