import {Anchor, Code, Group, Stack} from "@mantine/core";
import {Browser} from "@wailsio/runtime";
import {Facts} from "../../components/Facts";
import {StateBadge} from "../../components/StateBadge";
import {ago, bytes, lifetime, moment} from "../format";
import type {VM} from "../types";

/** What a VM is and where it is. */
export function VMOverview({vm}: {vm: VM}) {
  const urls = vm.urls ?? [];

  return (
    <Facts
      facts={[
        ["Name", vm.metadata.name],
        ["UUID", <Code key="uuid">{vm.metadata.uuid}</Code>],
        ["Slug", vm.metadata.slug],
        ["Kind", vm.docker ? "Docker VM" : "Machine"],
        ["State", <StateBadge key="state" state={vm.status.state} reason={vm.status.reason} />],
        ["Image", vm.spec.image],
        ["vCPUs", vm.spec.resources.cpus],
        ["Memory", bytes(vm.spec.resources.memory)],
        ["Disk", `${bytes(vm.spec.resources.disk)}${vm.spec.persistent_disk ? ", kept across restarts" : ", as its image left it on every start"}`],
        ["Ports", (vm.spec.ports ?? []).join(", ")],
        [
          "Served at",
          urls.length > 0 && (
            <Stack gap={2} key="urls">
              {urls.map((url) => (
                <Anchor key={url.port} size="sm" onClick={() => void Browser.OpenURL(url.url)}>
                  {url.url}
                </Anchor>
              ))}
            </Stack>
          ),
        ],
        ["Network", `ingress ${vm.spec.network.ingress}, egress ${vm.spec.network.egress}`],
        ["Kept", lifetime(vm.metadata.lifetime)],
        ["Expires", vm.metadata.expires_at && `${moment(vm.metadata.expires_at)} (${ago(vm.metadata.expires_at)})`],
        ["Node", vm.metadata.node],
        ["From the snapshot", vm.spec.source?.snapshot],
        ["Started", vm.status.started_at && ago(vm.status.started_at)],
        ["Created", vm.metadata.created_at && `${moment(vm.metadata.created_at)} (${ago(vm.metadata.created_at)})`],
        [
          "Labels",
          vm.metadata.labels && Object.keys(vm.metadata.labels).length > 0 && (
            <Group gap={4} key="labels">
              {Object.entries(vm.metadata.labels).map(([key, value]) => (
                <Code key={key}>
                  {key}={value}
                </Code>
              ))}
            </Group>
          ),
        ],
      ]}
    />
  );
}
