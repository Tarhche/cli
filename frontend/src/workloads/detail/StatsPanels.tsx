import {SimpleGrid, Stack, Text, Title} from "@mantine/core";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {FailureAlert} from "../../components/FailureAlert";
import {ago, bytes, share} from "../format";
import type {ContainerStats, Usage, VM} from "../types";
import {Meter} from "./Meter";
import {UsageHistory} from "./UsageHistory";

/** How often a container's sample is read again, while it is shown. */
const SAMPLE = 5000;

/** How often what was used over the last hour is read again. */
const HISTORY = 30000;

/** What a VM uses now, as its node last sampled it, and over the last hour. */
export function VMStatsPanel({vm, active}: {vm: VM; active: boolean}) {
  const uuid = vm.metadata.uuid ?? "";
  const usage = useQuery(`vm-usage:${uuid}`, () => Workloads.VMUsage(uuid, {}) as Promise<Usage>, {interval: active ? HISTORY : 0, enabled: active});
  const stats = vm.status.stats;

  return (
    <Stack>
      {stats ? (
        <>
          <SimpleGrid cols={{base: 1, sm: 3}}>
            <Meter label="CPU" value={stats.cpu_percent} said={`${stats.cpu_percent.toFixed(1)}%`} />
            <Meter label="Memory" value={share(stats.memory_used, stats.memory_limit)} said={`${bytes(stats.memory_used)} of ${bytes(stats.memory_limit)}`} />
            <Meter label="Disk" value={share(stats.disk_used, stats.disk_total)} said={`${bytes(stats.disk_used)} of ${bytes(stats.disk_total)}`} />
          </SimpleGrid>
          <Text size="sm" c="dimmed">
            {bytes(stats.network_rx)} received and {bytes(stats.network_tx)} sent since it started; sampled {ago(stats.sampled_at)}.
          </Text>
        </>
      ) : (
        <Text c="dimmed">It uses nothing while it does not run.</Text>
      )}
      <Title order={4}>The last hour</Title>
      <FailureAlert failure={usage.failure} title="What it used could not be read" />
      <UsageHistory usage={usage.data} />
    </Stack>
  );
}

/** What a container uses now, and over the last hour. */
export function ContainerStatsPanel({vm, id, active}: {vm: string; id: string; active: boolean}) {
  const stats = useQuery(`container-stats:${vm}/${id}`, () => Workloads.ContainerStats(vm, id) as Promise<ContainerStats | null>, {
    interval: active ? SAMPLE : 0,
    enabled: active,
  });
  const usage = useQuery(`container-usage:${vm}/${id}`, () => Workloads.ContainerUsage(vm, id, {}) as Promise<Usage>, {
    interval: active ? HISTORY : 0,
    enabled: active,
  });
  const sample = stats.data;

  return (
    <Stack>
      <FailureAlert failure={stats.failure} title="What it uses could not be read" />
      {sample ? (
        <>
          <SimpleGrid cols={{base: 1, sm: 3}}>
            <Meter label="CPU" value={sample.cpu_percent} said={`${sample.cpu_percent.toFixed(1)}%`} />
            <Meter label="Memory" value={share(sample.memory_used, sample.memory_limit)} said={`${bytes(sample.memory_used)} of ${bytes(sample.memory_limit)}`} />
            <Meter label="Processes" value={0} said={`${sample.pids}`} />
          </SimpleGrid>
          <Text size="sm" c="dimmed">
            {bytes(sample.network_rx)} received and {bytes(sample.network_tx)} sent, {bytes(sample.block_read)} read and {bytes(sample.block_write)} written; sampled{" "}
            {ago(sample.sampled_at)}.
          </Text>
        </>
      ) : (
        !stats.loading && !stats.failure && <Text c="dimmed">It uses nothing while it does not run.</Text>
      )}
      <Title order={4}>The last hour</Title>
      <FailureAlert failure={usage.failure} title="What it used could not be read" />
      <UsageHistory usage={usage.data} />
    </Stack>
  );
}
