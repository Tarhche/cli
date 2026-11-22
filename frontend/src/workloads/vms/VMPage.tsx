import {ActionIcon, Button, Group, Loader, Menu, Tabs, Tooltip} from "@mantine/core";
import {IconBraces, IconCamera, IconDots, IconHistory, IconPlayerPlay, IconPlayerStop, IconRefresh, IconTrash} from "@tabler/icons-react";
import {useState} from "react";
import {useAbilities} from "../../access/access";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal, type Dependents} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {FilesPanel} from "../detail/FilesPanel";
import {LogsPanel} from "../detail/LogsPanel";
import {PortsPanel} from "../detail/PortsPanel";
import {VMStatsPanel} from "../detail/StatsPanels";
import {TerminalPanel} from "../detail/TerminalPanel";
import {canStart, canStop} from "../format";
import {POLL} from "../listing";
import type {Logs, VM} from "../types";
import {RestoreSnapshotModal, TakeSnapshotModal} from "./SnapshotModals";
import {VMOverview} from "./VMOverview";

/** A tab of a workload's page. */
type Tab = "overview" | "logs" | "terminal" | "stats" | "files" | "ports";

/**
 * One of one's VMs: what it is, its log, a terminal in it, what it uses, its
 * files and its ports, each as far as the session may, and what it may do
 * with it.
 */
export function VMPage({uuid}: {uuid: string}) {
  const {back} = useNavigation();
  const act = useAct();
  const abilities = useAbilities();
  const may = abilities.vms;
  const vm = useQuery(`vm:${uuid}`, () => Workloads.VM(uuid) as Promise<VM>, {interval: POLL});
  const [tab, setTab] = useState<Tab>("overview");
  const [visited, setVisited] = useState<Set<Tab>>(() => new Set<Tab>(["overview"]));
  const [dialog, setDialog] = useState<"inspect" | "delete" | "snapshot" | "restore" | null>(null);
  const shown = vm.data;

  if (shown === undefined) {
    return (
      <>
        <PageHeader title="VM" onBack={back} />
        {vm.loading ? <Loader /> : <FailureAlert failure={vm.failure} title="The VM could not be read" />}
      </>
    );
  }

  const name = shown.metadata.name ?? uuid;
  const state = shown.status.state;
  const target = {vm: uuid};
  const offered: Record<Tab, boolean> = {
    overview: true,
    logs: may.logs,
    terminal: may.attach,
    stats: may.usage,
    files: may.exec,
    ports: may.forward || (shown.docker === true && may.exec),
  };
  const current = offered[tab] ? tab : "overview";
  const offersMore = abilities.snapshots.create || may.restore || may.delete;
  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(vm.reload);
  const open = (next: string | null) => {
    const chosen = (next ?? "overview") as Tab;
    setTab(chosen);
    setVisited((was) => new Set(was).add(chosen));
  };

  return (
    <>
      <PageHeader
        title={name}
        description={`${shown.docker ? "Docker VM" : "Machine"} · ${shown.metadata.slug ?? uuid}`}
        onBack={back}
        actions={
          <Group gap="xs">
            <StateBadge state={state} reason={shown.status.reason} />
            {may.start && canStart(state) && (
              <Button size="xs" variant="light" color="green" leftSection={<IconPlayerPlay size={14} />} onClick={() => command(`Starting ${name}`, () => Workloads.StartVM(uuid))}>
                Start
              </Button>
            )}
            {may.stop && canStop(state) && (
              <Button size="xs" variant="light" color="orange" leftSection={<IconPlayerStop size={14} />} onClick={() => command(`Stopping ${name}`, () => Workloads.StopVM(uuid))}>
                Stop
              </Button>
            )}
            {may.restart && canStop(state) && (
              <Button size="xs" variant="light" leftSection={<IconRefresh size={14} />} onClick={() => command(`Restarting ${name}`, () => Workloads.RestartVM(uuid))}>
                Restart
              </Button>
            )}
            {offersMore ? (
              <Menu position="bottom-end" withinPortal>
                <Menu.Target>
                  <ActionIcon variant="default" size="md" aria-label="More">
                    <IconDots size={16} />
                  </ActionIcon>
                </Menu.Target>
                <Menu.Dropdown>
                  {abilities.snapshots.create && (
                    <Menu.Item leftSection={<IconCamera size={14} />} onClick={() => setDialog("snapshot")}>
                      Take a snapshot
                    </Menu.Item>
                  )}
                  {may.restore && (
                    <Menu.Item leftSection={<IconHistory size={14} />} onClick={() => setDialog("restore")}>
                      Restore a snapshot
                    </Menu.Item>
                  )}
                  <Menu.Item leftSection={<IconBraces size={14} />} onClick={() => setDialog("inspect")}>
                    Inspect
                  </Menu.Item>
                  {may.delete && (
                    <>
                      <Menu.Divider />
                      <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => setDialog("delete")}>
                        Delete
                      </Menu.Item>
                    </>
                  )}
                </Menu.Dropdown>
              </Menu>
            ) : (
              <Tooltip label="Inspect">
                <ActionIcon variant="default" size="md" aria-label="Inspect" onClick={() => setDialog("inspect")}>
                  <IconBraces size={16} />
                </ActionIcon>
              </Tooltip>
            )}
          </Group>
        }
      />
      <FailureAlert failure={vm.failure} title="The VM could not be read again" />
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
          <VMOverview vm={shown} />
        </Tabs.Panel>
        <Tabs.Panel value="logs" pt="md">
          {offered.logs && visited.has("logs") && <LogsPanel id={`vm:${uuid}`} read={(query) => Workloads.VMLogs(uuid, query) as Promise<Logs>} active={current === "logs"} />}
        </Tabs.Panel>
        <Tabs.Panel value="terminal" pt="md">
          {offered.terminal && visited.has("terminal") && <TerminalPanel target={target} />}
        </Tabs.Panel>
        <Tabs.Panel value="stats" pt="md">
          {offered.stats && visited.has("stats") && <VMStatsPanel vm={shown} active={current === "stats"} />}
        </Tabs.Panel>
        <Tabs.Panel value="files" pt="md">
          {offered.files && visited.has("files") && <FilesPanel target={target} />}
        </Tabs.Panel>
        <Tabs.Panel value="ports" pt="md">
          {offered.ports && visited.has("ports") && (
            <PortsPanel target={target} ports={may.forward ? (shown.spec.ports ?? []) : []} hint={portsHint(shown.docker === true, may.forward, may.exec)} />
          )}
        </Tabs.Panel>
      </Tabs>
      <JsonDrawer opened={dialog === "inspect"} onClose={() => setDialog(null)} title={name} value={shown} />
      <TakeSnapshotModal opened={dialog === "snapshot"} onClose={() => setDialog(null)} vm={{uuid, name}} />
      <RestoreSnapshotModal opened={dialog === "restore"} onClose={() => setDialog(null)} vm={{uuid, name}} />
      <DeleteModal
        opened={dialog === "delete"}
        onClose={() => setDialog(null)}
        title="Delete VM"
        name={name}
        description="Its disk goes with it; its snapshots stay."
        dependents={() => Workloads.VMDependents(uuid) as Promise<Dependents>}
        onDelete={({cascade}) => Workloads.DeleteVM(uuid, cascade)}
        onDeleted={back}
      />
    </>
  );
}

/**
 * Which of a VM's ports the session may forward: those it publishes, under
 * the forward permission, and a Docker VM's others, relayed from inside it
 * under the exec one.
 */
function portsHint(docker: boolean, forward: boolean, exec: boolean): string {
  if (!docker || !exec) {
    return "A VM forwards the ports it publishes.";
  }

  return forward ? "A VM forwards the ports it publishes, and a Docker VM relays any other from inside it." : "A Docker VM relays the ports it does not publish from inside it.";
}
