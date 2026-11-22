import {Badge, Button, EmptyState, Group, Loader, Pagination, Table, Text} from "@mantine/core";
import {IconPlus, IconServer2} from "@tabler/icons-react";
import {useState} from "react";
import {useAbilities} from "../../access/access";
import {Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {DeleteModal, type Dependents} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {confirmRestart, confirmStop} from "../confirm";
import {ago, sizeOf} from "../format";
import {POLL} from "../listing";
import type {Page, VM} from "../types";
import {WorkloadActions} from "../WorkloadActions";

/** One's VMs: machines and Docker VMs, and what the session may do with them. */
export function VMsPage() {
  const {navigate} = useNavigation();
  const act = useAct();
  const {vms: may} = useAbilities();
  const [page, setPage] = useState(1);
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
          may.create && (
            <Button leftSection={<IconPlus size={16} />} onClick={() => navigate({page: "create-vm"})}>
              Create VM
            </Button>
          )
        }
      />
      <FailureAlert failure={vms.failure} title="Your VMs could not be read" />
      {vms.loading && <Loader />}
      {!vms.loading && items.length === 0 && !vms.failure && (
        <EmptyState icon={<IconServer2 />} title="No VMs yet" description={may.create ? "Create one to run something, or to run containers in." : undefined} />
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
                const name = vm.metadata.name ?? uuid;

                return (
                  <Table.Tr key={uuid} style={may.show ? {cursor: "pointer"} : undefined} onClick={may.show ? () => navigate({page: "vm", uuid}) : undefined}>
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
                        onStart={may.start ? () => command(`Starting ${name}`, () => Workloads.StartVM(uuid)) : undefined}
                        onStop={may.stop ? () => confirmStop("vm", name, () => command(`Stopping ${name}`, () => Workloads.StopVM(uuid))) : undefined}
                        onRestart={may.restart ? () => confirmRestart("vm", name, () => command(`Restarting ${name}`, () => Workloads.RestartVM(uuid))) : undefined}
                        onDelete={may.delete ? () => setDeleted(vm) : undefined}
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
