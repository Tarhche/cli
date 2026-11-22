import {Button, EmptyState, Loader, Table, Text} from "@mantine/core";
import {IconBox, IconPlus} from "@tabler/icons-react";
import {useState} from "react";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {ago, portsOf} from "../format";
import {POLL} from "../listing";
import type {Container, Items} from "../types";
import {WorkloadActions} from "../WorkloadActions";

/** What identifies a container in its Docker VM: its uuid, else its Docker id. */
function idOf(container: Container): string {
  return container.metadata.uuid || container.status.id || container.status.name || "";
}

/** One's containers, across one's Docker VMs. */
export function ContainersPage() {
  const {navigate} = useNavigation();
  const act = useAct();
  const [inspected, setInspected] = useState<Container | null>(null);
  const [deleted, setDeleted] = useState<Container | null>(null);
  const containers = useQuery("containers", () => Workloads.Containers("") as Promise<Items<Container>>, {interval: POLL});
  const items = containers.data?.items ?? [];

  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(containers.reload);

  return (
    <>
      <PageHeader
        title="Containers"
        description="What runs in your Docker VMs."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={() => navigate({page: "run-container"})}>
            Run container
          </Button>
        }
      />
      <FailureAlert failure={containers.failure} title="Your containers could not be read" />
      {containers.loading && <Loader />}
      {!containers.loading && items.length === 0 && !containers.failure && (
        <EmptyState icon={<IconBox />} title="No containers yet" description="Run one, in a Docker VM of yours or a new one." />
      )}
      {items.length > 0 && (
        <Table.ScrollContainer minWidth={760}>
          <Table highlightOnHover verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Image</Table.Th>
                <Table.Th>State</Table.Th>
                <Table.Th>Ports</Table.Th>
                <Table.Th>VM</Table.Th>
                <Table.Th>Created</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((container) => {
                const id = idOf(container);
                const vm = container.vm_uuid;
                const name = container.status.name || container.spec.name || container.metadata.name || id;

                return (
                  <Table.Tr key={`${vm}/${id}`} style={{cursor: "pointer"}} onClick={() => navigate({page: "container", vm, id})}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {name}
                      </Text>
                      <Text size="xs" c="dimmed">
                        {container.status.docker_status}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" truncate maw={220}>
                        {container.spec.image}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <StateBadge state={container.status.state} reason={container.status.failure?.message ?? container.status.reason} />
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{portsOf(container.status.ports ?? container.spec.ports)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{container.vm_name}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{ago(container.metadata.created_at)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <WorkloadActions
                        state={container.status.state}
                        onStart={() => command(`Starting ${name}`, () => Workloads.StartContainer(vm, id))}
                        onStop={() => command(`Stopping ${name}`, () => Workloads.StopContainer(vm, id))}
                        onRestart={() => command(`Restarting ${name}`, () => Workloads.RestartContainer(vm, id))}
                        onInspect={() => setInspected(container)}
                        onDelete={() => setDeleted(container)}
                      />
                    </Table.Td>
                  </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected?.status.name ?? "Container"} value={inspected} />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete container"
        name={deleted?.status.name || deleted?.spec.name || (deleted ? idOf(deleted) : "")}
        force="Delete it though it runs"
        onDelete={({force}) => Workloads.DeleteContainer(deleted?.vm_uuid ?? "", deleted ? idOf(deleted) : "", force)}
        onDeleted={containers.reload}
      />
    </>
  );
}
