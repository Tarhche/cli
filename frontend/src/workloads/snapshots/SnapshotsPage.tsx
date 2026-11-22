import {ActionIcon, Badge, EmptyState, Group, Loader, Pagination, Table, Text, Tooltip} from "@mantine/core";
import {IconCamera, IconCopyPlus, IconPencil, IconRestore} from "@tabler/icons-react";
import {useState} from "react";
import {useAbilities} from "../../access/access";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {DeleteModal} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {ago, bytes} from "../format";
import {POLL} from "../listing";
import type {Page, Snapshot} from "../types";
import {WorkloadActions} from "../WorkloadActions";
import {RenameModal} from "./RenameModal";
import {RestoreOntoModal} from "./RestoreOntoModal";

/** One's snapshots: VMs' disks, kept to restore as a new VM or onto one, and what the session may do with them. */
export function SnapshotsPage() {
  const {navigate} = useNavigation();
  const abilities = useAbilities();
  const may = abilities.snapshots;
  const takes = may.create && abilities.vms.show;
  const [page, setPage] = useState(1);
  const [restored, setRestored] = useState<Snapshot | null>(null);
  const [renamed, setRenamed] = useState<Snapshot | null>(null);
  const [deleted, setDeleted] = useState<Snapshot | null>(null);
  const snapshots = useQuery(`snapshots:${page}`, () => Workloads.Snapshots(page, "") as Promise<Page<Snapshot>>, {interval: POLL});
  const items = snapshots.data?.items ?? [];
  const pages = snapshots.data?.pagination?.total_pages ?? 0;

  return (
    <>
      <PageHeader title="Snapshots" description={`VMs' disks as they were, to make a VM from or restore one to.${takes ? " A snapshot is taken on its VM's page." : ""}`} />
      <FailureAlert failure={snapshots.failure} title="Your snapshots could not be read" />
      {snapshots.loading && <Loader />}
      {!snapshots.loading && items.length === 0 && !snapshots.failure && <EmptyState icon={<IconCamera />} title="No snapshots yet" description={takes ? "Take one on a VM's page." : undefined} />}
      {items.length > 0 && (
        <Table.ScrollContainer minWidth={720}>
          <Table verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>State</Table.Th>
                <Table.Th>Of</Table.Th>
                <Table.Th>Size</Table.Th>
                <Table.Th>Taken</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((snapshot) => {
                const uuid = snapshot.metadata.uuid ?? "";
                const ready = snapshot.status.state === "ready";

                return (
                  <Table.Tr key={uuid}>
                    <Table.Td>
                      <Group gap="xs">
                        <Text size="sm" fw={500}>
                          {snapshot.metadata.name}
                        </Text>
                        <Badge size="xs" variant="outline" color={snapshot.docker ? "cyan" : "gray"}>
                          {snapshot.docker ? "docker" : "machine"}
                        </Badge>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <StateBadge state={snapshot.status.state} reason={snapshot.status.reason} />
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{snapshot.spec.vm.name || snapshot.spec.vm.uuid}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{bytes(snapshot.status.size)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm">{ago(snapshot.metadata.created_at)}</Text>
                    </Table.Td>
                    <Table.Td>
                      <Group gap={4} justify="flex-end" wrap="nowrap">
                        {abilities.vms.create && (
                          <Tooltip label="Restore as new VM">
                            <ActionIcon variant="subtle" color="teal" aria-label="Restore as new VM" disabled={!ready} onClick={() => navigate({page: "create-vm", snapshot: uuid})}>
                              <IconCopyPlus size={16} />
                            </ActionIcon>
                          </Tooltip>
                        )}
                        {abilities.vms.restore && (
                          <Tooltip label="Restore onto a VM">
                            <ActionIcon variant="subtle" color="orange" aria-label="Restore onto a VM" disabled={!ready} onClick={() => setRestored(snapshot)}>
                              <IconRestore size={16} />
                            </ActionIcon>
                          </Tooltip>
                        )}
                        {may.update && (
                          <Tooltip label="Rename">
                            <ActionIcon variant="subtle" color="gray" aria-label="Rename" onClick={() => setRenamed(snapshot)}>
                              <IconPencil size={16} />
                            </ActionIcon>
                          </Tooltip>
                        )}
                        <WorkloadActions onDelete={may.delete ? () => setDeleted(snapshot) : undefined} />
                      </Group>
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
      {restored && <RestoreOntoModal opened onClose={() => setRestored(null)} snapshot={restored} />}
      <RenameModal
        opened={renamed !== null}
        onClose={() => setRenamed(null)}
        snapshot={{uuid: renamed?.metadata.uuid ?? "", name: renamed?.metadata.name ?? ""}}
        onRenamed={snapshots.reload}
      />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete snapshot"
        name={deleted?.metadata.name ?? ""}
        description="It cannot be restored once it is gone."
        onDelete={() => Workloads.DeleteSnapshot(deleted?.metadata.uuid ?? "")}
        onDeleted={snapshots.reload}
      />
    </>
  );
}
