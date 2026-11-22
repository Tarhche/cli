import {Badge, Button, EmptyState, Group, Loader, Pagination, Table, Text} from "@mantine/core";
import {IconPlus, IconServer2} from "@tabler/icons-react";
import {useState} from "react";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal, type Dependents} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {ago, sizeOf} from "../format";
import {POLL} from "../listing";
import type {Page, VM} from "../types";
import {WorkloadActions} from "../WorkloadActions";

/** One's VMs: machines and Docker VMs. */
export function VMsPage() {
  const {navigate} = useNavigation();
  const act = useAct();
  const [page, setPage] = useState(1);
  const [inspected, setInspected] = useState<VM | null>(null);
  const [deleted, setDeleted] = useState<VM | null>(null);
  const vms = useQuery(`vms:${page}`, () => Workloads.VMs(page) as Promise<Page<VM>>, {interval: POLL});
  const items = vms.data?.items ?? [];
  const pages = vms.data?.pagination?.total_pages ?? 0;

  const command = (title: string, run: () => Promise<unknown>) => void act(title, run).then(vms.reload);

  return (
    <>
      <PageHeader
        title="VMs"
        description="Machines a terminal is opened in, and Docker VMs containers and stacks run in."
        actions={
          <Button leftSection={<IconPlus size={16} />} onClick={() => navigate({page: "create-vm"})}>
            Create VM
          </Button>
        }
      />
      <FailureAlert failure={vms.failure} title="Your VMs could not be read" />
      {vms.loading && <Loader />}
      {!vms.loading && items.length === 0 && !vms.failure && (
        <EmptyState icon={<IconServer2 />} title="No VMs yet" description="Create one to run something, or to run containers in." />
      )}
      {items.length > 0 && (
        <Table.ScrollContainer minWidth={720}>
          <Table highlightOnHover verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Kind</Table.Th>
                <Table.Th>State</Table.Th>
                <Table.Th>Image</Table.Th>
                <Table.Th>Size</Table.Th>
                <Table.Th>Created</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((vm) => {
                const uuid = vm.metadata.uuid ?? "";

                return (
                  <Table.Tr key={uuid} style={{cursor: "pointer"}} onClick={() => navigate({page: "vm", uuid})}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {vm.metadata.name}
                      </Text>
                      <Text size="xs" c="dimmed">
                        {vm.metadata.slug}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge variant="outline" color={vm.docker ? "cyan" : "gray"}>
                        {vm.docker ? "docker" : "machine"}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <StateBadge state={vm.status.state} reason={vm.status.reason} />
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" truncate maw={220}>
                        {vm.spec.image}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{sizeOf(vm.spec.resources)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{ago(vm.metadata.created_at)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <WorkloadActions
                        state={vm.status.state}
                        onStart={() => command(`Starting ${vm.metadata.name}`, () => Workloads.StartVM(uuid))}
                        onStop={() => command(`Stopping ${vm.metadata.name}`, () => Workloads.StopVM(uuid))}
                        onRestart={() => command(`Restarting ${vm.metadata.name}`, () => Workloads.RestartVM(uuid))}
                        onInspect={() => setInspected(vm)}
                        onDelete={() => setDeleted(vm)}
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
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected?.metadata.name ?? ""} value={inspected} />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete VM"
        name={deleted?.metadata.name ?? ""}
        description="Its disk goes with it; its snapshots stay."
        dependents={() => Workloads.VMDependents(deleted?.metadata.uuid ?? "") as Promise<Dependents>}
        onDelete={({cascade}) => Workloads.DeleteVM(deleted?.metadata.uuid ?? "", cascade)}
        onDeleted={vms.reload}
      />
    </>
  );
}
