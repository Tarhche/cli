import {ActionIcon, Badge, EmptyState, Group, Loader, Pagination, Table, Text, Tooltip} from "@mantine/core";
import {IconCamera, IconPencil, IconServer2} from "@tabler/icons-react";
import {useState} from "react";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {DeleteModal} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {ago, bytes} from "../format";
import {POLL} from "../listing";
import type {Page, Snapshot} from "../types";
import {WorkloadActions} from "../WorkloadActions";
import {RenameModal} from "./RenameModal";

/** One's snapshots: VMs' disks, kept to make a VM from or to restore one to. */
export function SnapshotsPage() {
  const {navigate} = useNavigation();
  const [page, setPage] = useState(1);
  const [inspected, setInspected] = useState<Snapshot | null>(null);
  const [renamed, setRenamed] = useState<Snapshot | null>(null);
  const [deleted, setDeleted] = useState<Snapshot | null>(null);
  const snapshots = useQuery(`snapshots:${page}`, () => Workloads.Snapshots(page, "") as Promise<Page<Snapshot>>, {interval: POLL});
  const items = snapshots.data?.items ?? [];
  const pages = snapshots.data?.pagination?.total_pages ?? 0;

  return (
    <>
      <PageHeader title="Snapshots" description="VMs' disks as they were, to make a VM from or restore one to. A snapshot is taken on its VM's page." />
      <FailureAlert failure={snapshots.failure} title="Your snapshots could not be read" />
      {snapshots.loading && <Loader />}
      {!snapshots.loading && items.length === 0 && !snapshots.failure && <EmptyState icon={<IconCamera />} title="No snapshots yet" description="Take one on a VM's page." />}
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
                        <Tooltip label="Make a VM from it">
                          <ActionIcon variant="subtle" aria-label="Make a VM from it" disabled={!ready} onClick={() => navigate({page: "create-vm", snapshot: uuid})}>
                            <IconServer2 size={16} />
                          </ActionIcon>
                        </Tooltip>
                        <Tooltip label="Rename">
                          <ActionIcon variant="subtle" color="gray" aria-label="Rename" onClick={() => setRenamed(snapshot)}>
                            <IconPencil size={16} />
                          </ActionIcon>
                        </Tooltip>
                        <WorkloadActions onInspect={() => setInspected(snapshot)} onDelete={() => setDeleted(snapshot)} />
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
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected?.metadata.name ?? "Snapshot"} value={inspected} />
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
        description="What it kept goes with it."
        onDelete={() => Workloads.DeleteSnapshot(deleted?.metadata.uuid ?? "")}
        onDeleted={snapshots.reload}
      />
    </>
  );
}
