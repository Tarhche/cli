import {ActionIcon, Button, Group, Loader, Menu, Tabs} from "@mantine/core";
import {IconDots, IconPlayerPlay, IconPlayerStop, IconRefresh, IconTrash} from "@tabler/icons-react";
import {useState} from "react";
import {useAbilities} from "../../access/access";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {FilesPanel} from "../detail/FilesPanel";
import {LogsPanel} from "../detail/LogsPanel";
import {PortsPanel} from "../detail/PortsPanel";
import {ContainerStatsPanel} from "../detail/StatsPanels";
import {TerminalPanel} from "../detail/TerminalPanel";
import {canStart, canStop} from "../format";
import {POLL} from "../listing";
import type {Container, Logs} from "../types";
import {ContainerOverview} from "./ContainerOverview";

type Tab = "overview" | "logs" | "terminal" | "stats" | "files" | "ports";

/**
 * One of one's containers, in its Docker VM: what it is, its log, a shell in
 * it, what it uses, its files and its ports, each as far as the session
 * may, and what it may do with it.
 */
export function ContainerPage({vm, id}: {vm: string; id: string}) {
  const {back} = useNavigation();
  const act = useAct();
  const {containers: may} = useAbilities();
  const container = useQuery(`container:${vm}/${id}`, () => Workloads.Container(vm, id) as Promise<Container>, {interval: POLL});
  const [tab, setTab] = useState<Tab>("overview");
  const [visited, setVisited] = useState<Set<Tab>>(() => new Set<Tab>(["overview"]));
  const [dialog, setDialog] = useState<"delete" | null>(null);
  const shown = container.data;

  if (shown === undefined) {
    return (
      <>
        <PageHeader title="Container" onBack={back} />
        {container.loading ? <Loader /> : <FailureAlert failure={container.failure} title="The container could not be read" />}
      </>
    );
  }

  const name = shown.status.name || shown.spec.name || id;
  const state = shown.status.state;
  const target = {vm, container: id};
  const offered: Record<Tab, boolean> = {overview: true, logs: may.logs, terminal: may.exec, stats: may.usage, files: may.exec, ports: may.exec};
  const current = offered[tab] ? tab : "overview";
  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(container.reload);
  const open = (next: string | null) => {
    const chosen = (next ?? "overview") as Tab;
    setTab(chosen);
    setVisited((was) => new Set(was).add(chosen));
  };

  return (
    <>
      <PageHeader
        title={name}
        description={`${shown.spec.image} · in ${shown.vm_name || vm}`}
        onBack={back}
        actions={
          <Group gap="xs">
            <StateBadge state={state} reason={shown.status.failure?.message ?? shown.status.reason} />
            {may.start && canStart(state) && (
              <Button size="xs" variant="light" color="green" leftSection={<IconPlayerPlay size={14} />} onClick={() => command(`Starting ${name}`, () => Workloads.StartContainer(vm, id))}>
                Start
              </Button>
            )}
            {may.stop && canStop(state) && (
              <Button size="xs" variant="light" color="orange" leftSection={<IconPlayerStop size={14} />} onClick={() => command(`Stopping ${name}`, () => Workloads.StopContainer(vm, id))}>
                Stop
              </Button>
            )}
            {may.restart && canStop(state) && (
              <Button size="xs" variant="light" leftSection={<IconRefresh size={14} />} onClick={() => command(`Restarting ${name}`, () => Workloads.RestartContainer(vm, id))}>
                Restart
              </Button>
            )}
            {may.delete && (
              <Menu position="bottom-end" withinPortal>
                <Menu.Target>
                  <ActionIcon variant="default" size="md" aria-label="More">
                    <IconDots size={16} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => setDialog("delete")}>
                    Delete
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            )}
          </Group>
        }
      />
      <FailureAlert failure={container.failure} title="The container could not be read again" />
      <Tabs value={current} onChange={open}>
        <Tabs.List>
          <Tabs.Tab value="overview">Overview</Tabs.Tab>
          {offered.logs && <Tabs.Tab value="logs">Logs</Tabs.Tab>}
          {offered.terminal && <Tabs.Tab value="terminal">Terminal</Tabs.Tab>}
          {offered.stats && <Tabs.Tab value="stats">Stats</Tabs.Tab>}
          {offered.files && <Tabs.Tab value="files">Files</Tabs.Tab>}
          {offered.ports && <Tabs.Tab value="ports">Ports</Tabs.Tab>}
        </Tabs.List>
        <Tabs.Panel value="overview" pt="md">
          <ContainerOverview container={shown} />
        </Tabs.Panel>
        <Tabs.Panel value="logs" pt="md">
          {offered.logs && visited.has("logs") && (
            <LogsPanel id={`container:${vm}/${id}`} read={(query) => Workloads.ContainerLogs(vm, id, query) as Promise<Logs>} active={current === "logs"} />
          )}
        </Tabs.Panel>
        <Tabs.Panel value="terminal" pt="md">
          {offered.terminal && visited.has("terminal") && <TerminalPanel target={target} />}
        </Tabs.Panel>
        <Tabs.Panel value="stats" pt="md">
          {offered.stats && visited.has("stats") && <ContainerStatsPanel vm={vm} id={id} active={current === "stats"} />}
        </Tabs.Panel>
        <Tabs.Panel value="files" pt="md">
          {offered.files && visited.has("files") && <FilesPanel target={target} />}
        </Tabs.Panel>
        <Tabs.Panel value="ports" pt="md">
          {offered.ports && visited.has("ports") && (
            <PortsPanel target={target} ports={(shown.spec.ports ?? []).map((p) => p.container_port)} hint="A container's port is reached through its Docker VM, published or not." />
          )}
        </Tabs.Panel>
      </Tabs>
      <DeleteModal
        opened={dialog === "delete"}
        onClose={() => setDialog(null)}
        title="Delete container"
        name={name}
        force="Delete it though it runs"
        onDelete={({force}) => Workloads.DeleteContainer(vm, id, force)}
        onDeleted={back}
      />
    </>
  );
}
