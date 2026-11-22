import {Badge, Button, EmptyState, Group, Loader, Pagination, Table, Text, Tooltip} from "@mantine/core";
import {IconLayersIntersect, IconPlus} from "@tabler/icons-react";
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
import {confirmRestart, confirmStop} from "../confirm";
import {ago, stateColor} from "../format";
import {POLL} from "../listing";
import type {Page, Stack} from "../types";
import {WorkloadActions} from "../WorkloadActions";

/** One's stacks: services of a compose file, run as containers of a Docker VM, and what the session may do with them. */
export function StacksPage() {
  const {navigate} = useNavigation();
  const act = useAct();
  const {stacks: may} = useAbilities();
  const [page, setPage] = useState(1);
  const [deleted, setDeleted] = useState<Stack | null>(null);
  const stacks = useQuery(`stacks:${page}`, () => Workloads.Stacks(page) as Promise<Page<Stack>>, {interval: POLL});
  const items = stacks.data?.items ?? [];
  const pages = stacks.data?.pagination?.total_pages ?? 0;

  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(stacks.reload);

  return (
    <>
      <PageHeader
        title="Stacks"
        description="Services of a compose file, each a container of a Docker VM."
        actions={
          may.create && (
            <Button leftSection={<IconPlus size={16} />} onClick={() => navigate({page: "deploy-stack"})}>
              Deploy stack
            </Button>
          )
        }
      />
      <FailureAlert failure={stacks.failure} title="Your stacks could not be read" />
      {stacks.loading && <Loader />}
      {!stacks.loading && items.length === 0 && !stacks.failure && (
        <EmptyState icon={<IconLayersIntersect />} title="No stacks yet" description={may.create ? "Deploy one from a compose file." : undefined} />
      )}
      {items.length > 0 && (
        <Table.ScrollContainer minWidth={720}>
          <Table highlightOnHover verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>State</Table.Th>
                <Table.Th>Services</Table.Th>
                <Table.Th>VM</Table.Th>
                <Table.Th>Created</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((stack) => {
                const uuid = stack.metadata.uuid ?? "";
                const name = stack.metadata.name ?? uuid;

                return (
                  <Table.Tr key={uuid} style={may.show ? {cursor: "pointer"} : undefined} onClick={may.show ? () => navigate({page: "stack", uuid}) : undefined}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {name}
                      </Text>
                      <Text size="xs" c="dimmed">
                        {stack.metadata.slug}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <StateBadge state={stack.status.state} reason={stack.status.failure?.message ?? stack.status.reason} />
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4}>
                        {(stack.status.services ?? []).map((service) => (
                          <Tooltip key={service.name} label={`${service.name}: ${service.state}`}>
                            <Badge size="sm" variant="dot" color={stateColor(service.state)}>
                              {service.name}
                            </Badge>
                          </Tooltip>
                        ))}
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{stack.vm_name}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{ago(stack.metadata.created_at)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <WorkloadActions
                        state={stack.status.state}
                        onStart={may.start ? () => command(`Starting ${name}`, () => Workloads.StartStack(uuid)) : undefined}
                        onStop={may.stop ? () => confirmStop("stack", name, () => command(`Stopping ${name}`, () => Workloads.StopStack(uuid))) : undefined}
                        onRestart={may.restart ? () => confirmRestart("stack", name, () => command(`Restarting ${name}`, () => Workloads.RestartStack(uuid))) : undefined}
                        onDelete={may.delete ? () => setDeleted(stack) : undefined}
                      />
                    </Table.Td>
                  </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
      {pages > 1 && (
        <Group justify="center" mt="md">
          <Pagination total={pages} value={page} onChange={setPage} />
        </Group>
      )}
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete stack"
        name={deleted?.metadata.name ?? ""}
        description="Its containers and networks go with it; its volumes and its VM stay."
        onDelete={() => Workloads.DeleteStack(deleted?.metadata.uuid ?? "")}
        onDeleted={stacks.reload}
      />
    </>
  );
}
