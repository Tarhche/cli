import {Badge, Button, EmptyState, Group, Loader, Paper, Table, Text, TextInput} from "@mantine/core";
import {IconDatabase, IconPlus} from "@tabler/icons-react";
import {type FormEvent, useState} from "react";
import {useAbilities} from "../../access/access";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {DeleteModal, type Dependents} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {ago} from "../format";
import {POLL} from "../listing";
import type {Items, Volume} from "../types";
import {WorkloadActions} from "../WorkloadActions";
import {DockerVMSelect} from "./DockerVMSelect";

/** What a volume is called in its Docker VM. */
function nameOf(volume: Volume): string {
  return volume.spec.name || volume.status.name || volume.metadata.name || "";
}

/** The volumes of one of one's Docker VMs, and what the session may do with them. */
export function VolumesPage() {
  const {volumes: may} = useAbilities();
  const [vm, setVM] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [inspected, setInspected] = useState<Volume | null>(null);
  const [deleted, setDeleted] = useState<Volume | null>(null);
  const volumes = useQuery(`volumes:${vm}`, () => Workloads.Volumes(vm ?? "") as Promise<Items<Volume>>, {interval: POLL, enabled: vm !== null});
  const creating = useRun(async () => {
    await Workloads.CreateVolume(vm ?? "", {name: name.trim()});
    setName("");
    volumes.reload();
  });
  const laidOut = formFailure(creating.failure, (field) => (field === "name" || field === "driver" ? "name" : undefined));
  const items = volumes.data?.items ?? [];

  const create = (event: FormEvent) => {
    event.preventDefault();
    void creating.run();
  };

  return (
    <>
      <PageHeader title="Volumes" description="What the containers of a Docker VM keep on its disk." />
      <DockerVMSelect value={vm} onChange={setVM} />
      {vm !== null && (
        <>
          {may.create && (
            <Paper withBorder p="md" mb="md" component="form" onSubmit={create}>
              <Group align="flex-start">
                <TextInput placeholder="data" aria-label="Volume to create" value={name} onChange={(event) => setName(event.currentTarget.value)} error={laidOut.fields.name} style={{flex: 1}} />
                <Button type="submit" leftSection={<IconPlus size={16} />} loading={creating.pending}>
                  Create
                </Button>
              </Group>
              <FailureAlert messages={laidOut.messages} failure={creating.failure} title="The volume could not be created" />
            </Paper>
          )}
          <FailureAlert failure={volumes.failure} title="Its volumes could not be read" />
          {volumes.loading && <Loader />}
          {!volumes.loading && items.length === 0 && !volumes.failure && (
            <EmptyState icon={<IconDatabase />} title="No volumes" description={may.create ? "Create one for a container to keep what it writes." : undefined} />
          )}
          {items.length > 0 && (
            <Table.ScrollContainer minWidth={560}>
              <Table verticalSpacing="sm">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Name</Table.Th>
                    <Table.Th>State</Table.Th>
                    <Table.Th>Driver</Table.Th>
                    <Table.Th>Created</Table.Th>
                    <Table.Th />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {items.map((volume) => (
                    <Table.Tr key={volume.metadata.uuid ?? nameOf(volume)}>
                      <Table.Td>
                        <Group gap="xs">
                          <Text size="sm" fw={500}>
                            {nameOf(volume)}
                          </Text>
                          {volume.status.in_use && (
                            <Badge size="xs" variant="light">
                              in use
                            </Badge>
                          )}
                        </Group>
                      </Table.Td>
                      <Table.Td>
                        <StateBadge state={volume.status.state} reason={volume.status.failure?.message ?? volume.status.reason} />
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{volume.status.driver || volume.spec.driver || "local"}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{ago(volume.status.created_at)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <WorkloadActions onInspect={() => setInspected(volume)} onDelete={may.delete ? () => setDeleted(volume) : undefined} />
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </>
      )}
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected ? nameOf(inspected) : "Volume"} value={inspected} />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete volume"
        name={deleted ? nameOf(deleted) : ""}
        description="What is in it goes with it."
        dependents={() => Workloads.VolumeDependents(vm ?? "", deleted ? nameOf(deleted) : "") as Promise<Dependents>}
        force="Delete it though a container uses it"
        onDelete={({cascade, force}) => Workloads.DeleteVolume(vm ?? "", deleted ? nameOf(deleted) : "", cascade, force)}
        onDeleted={volumes.reload}
      />
    </>
  );
}
