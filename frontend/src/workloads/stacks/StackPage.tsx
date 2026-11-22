import {ActionIcon, Alert, Anchor, Button, Code, Collapse, Group, Loader, Menu, Paper, Stack, Table, Text, Title} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {IconAlertTriangle, IconClockHour4, IconDots, IconPlayerPlay, IconPlayerStop, IconRefresh, IconTrash} from "@tabler/icons-react";
import {useId, useState} from "react";
import {useAbilities} from "../../access/access";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal} from "../../components/DeleteModal";
import {Facts} from "../../components/Facts";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {confirmRestart, confirmStop} from "../confirm";
import {ago, canStart, canStop, moment} from "../format";
import {POLL} from "../listing";
import type {Stack as StackResource, VM} from "../types";

/** How a stack says why it is where it is, in the states it says so in. */
type Reasoned = {
  title: string;
  color: string;
  icon: typeof IconAlertTriangle;

  /** Whether it is worth interrupting for. */
  urgent: boolean;
};

const REASONED: Record<string, Reasoned> = {
  waiting: {title: "What it is waiting for", color: "yellow", icon: IconClockHour4, urgent: false},
  degraded: {title: "What is not running", color: "orange", icon: IconAlertTriangle, urgent: false},
  failed: {title: "Why it failed", color: "red", icon: IconAlertTriangle, urgent: true},
};

/** The Docker VM a stack lives in: the one among its refs, or, before its graph made one for it, the one its spec names. */
function vmOf(stack: StackResource): string {
  const {refs, owners} = stack.metadata;

  return refs?.find((r) => r.kind === "vm")?.uuid ?? owners?.find((r) => r.kind === "vm")?.uuid ?? stack.spec.vm.uuid ?? "";
}

/**
 * One of one's stacks: where it is and why when it says, the Docker VM it
 * lives in, its compose file, folded, and the containers it has now. It is
 * started, stopped, restarted or deleted, never changed: a different file is
 * a different stack.
 */
export function StackPage({uuid}: {uuid: string}) {
  const {navigate, back} = useNavigation();
  const act = useAct();
  const abilities = useAbilities();
  const may = abilities.stacks;
  const stack = useQuery(`stack:${uuid}`, () => Workloads.Stack(uuid) as Promise<StackResource>, {interval: POLL});
  const vmUUID = stack.data ? vmOf(stack.data) : "";
  const vm = useQuery(`vm:${vmUUID}`, () => Workloads.VM(vmUUID) as Promise<VM>, {interval: POLL, enabled: abilities.vms.show && vmUUID !== ""});
  const [deleting, setDeleting] = useState(false);
  const [compose, {toggle: toggleCompose}] = useDisclosure(false);
  const composeId = useId();
  const shown = stack.data;

  if (shown === undefined) {
    return (
      <>
        <PageHeader title="Stack" onBack={back} />
        {stack.loading ? <Loader /> : <FailureAlert failure={stack.failure} title="The stack could not be read" />}
      </>
    );
  }

  const name = shown.metadata.name ?? uuid;
  const state = shown.status.state;
  const why = shown.status.reason ? REASONED[state] : undefined;
  const vmName = vm.data?.metadata.name ?? shown.vm_name ?? shown.spec.vm.name ?? vmUUID;
  const containers = (shown.status.services ?? []).flatMap((service) => (service.containers ?? []).map((container) => ({service, container})));
  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(stack.reload);

  return (
    <>
      <PageHeader
        title={name}
        description={`Stack · ${shown.metadata.slug ?? uuid}`}
        onBack={back}
        actions={
          <Group gap="xs">
            <StateBadge state={state} reason={shown.status.failure?.message ?? shown.status.reason} />
            {may.start && canStart(state) && (
              <Button size="xs" variant="light" color="green" leftSection={<IconPlayerPlay size={14} />} onClick={() => command(`Starting ${name}`, () => Workloads.StartStack(uuid))}>
                Start
              </Button>
            )}
            {may.stop && canStop(state) && (
              <Button
                size="xs"
                variant="light"
                color="orange"
                leftSection={<IconPlayerStop size={14} />}
                onClick={() => confirmStop("stack", name, () => command(`Stopping ${name}`, () => Workloads.StopStack(uuid)))}
              >
                Stop
              </Button>
            )}
            {may.restart && canStop(state) && (
              <Button size="xs" variant="light" leftSection={<IconRefresh size={14} />} onClick={() => confirmRestart("stack", name, () => command(`Restarting ${name}`, () => Workloads.RestartStack(uuid)))}>
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
                  <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => setDeleting(true)}>
                    Delete
                  </Menu.Item>
                </Menu.Dropdown>
              </Menu>
            )}
          </Group>
        }
      />
      <Stack>
        <FailureAlert failure={stack.failure} title="The stack could not be read again" />
        {why && (
          <Alert color={why.color} variant="light" icon={<why.icon />} title={why.title} role={why.urgent ? "alert" : "status"}>
            {shown.status.reason}
          </Alert>
        )}
        <Facts
          facts={[
            [
              "VM",
              <Group gap="xs" key="vm">
                {abilities.vms.show && vmUUID !== "" ? (
                  <Anchor size="sm" onClick={() => navigate({page: "vm", uuid: vmUUID})}>
                    {vmName}
                  </Anchor>
                ) : (
                  <Text size="sm">{vmName}</Text>
                )}
                {vm.data && <StateBadge state={vm.data.status.state} reason={vm.data.status.reason} />}
              </Group>,
            ],
            ["Prefix", shown.metadata.slug && `what it makes is named ${shown.metadata.slug}_…`],
            ["Created", shown.metadata.created_at && `${moment(shown.metadata.created_at)} (${ago(shown.metadata.created_at)})`],
          ]}
        />
        {shown.spec.compose && (
          <Paper withBorder p="md">
            <Group justify="space-between">
              <Text fw={500}>Compose file</Text>
              <Button variant="subtle" size="compact-sm" onClick={toggleCompose} aria-expanded={compose} aria-controls={composeId}>
                {compose ? "Hide" : "Show"}
              </Button>
            </Group>
            <Collapse expanded={compose} id={composeId}>
              <Code block mt="sm">
                {shown.spec.compose}
              </Code>
            </Collapse>
          </Paper>
        )}
        <Title order={4}>Containers</Title>
        <Table.ScrollContainer minWidth={520}>
          <Table verticalSpacing="sm" striped withRowBorders>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Container</Table.Th>
                <Table.Th>Service</Table.Th>
                <Table.Th>State</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {containers.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={3} ta="center">
                    It has no containers
                  </Table.Td>
                </Table.Tr>
              )}
              {containers.map(({service, container}) => (
                <Table.Tr key={container.id}>
                  <Table.Td>
                    {abilities.containers.show && vmUUID !== "" ? (
                      <Anchor size="sm" onClick={() => navigate({page: "container", vm: vmUUID, id: container.id})}>
                        {container.name}
                      </Anchor>
                    ) : (
                      <Text size="sm">{container.name}</Text>
                    )}
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{service.name}</Text>
                  </Table.Td>
                  <Table.Td>
                    <StateBadge state={container.state} />
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      </Stack>
      <DeleteModal
        opened={deleting}
        onClose={() => setDeleting(false)}
        title="Delete stack"
        name={name}
        description="Its containers and networks go with it; its volumes and its VM stay."
        onDelete={() => Workloads.DeleteStack(uuid)}
        onDeleted={back}
      />
    </>
  );
}
