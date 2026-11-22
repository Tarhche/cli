import {Button, EmptyState, Group, Loader, Paper, Switch, Table, Text, TextInput} from "@mantine/core";
import {IconNetwork, IconPlus} from "@tabler/icons-react";
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
import type {DockerNetwork, Items} from "../types";
import {WorkloadActions} from "../WorkloadActions";
import {DockerVMSelect} from "./DockerVMSelect";

/** What identifies a network in its Docker VM: its uuid, else its name. */
function idOf(network: DockerNetwork): string {
  return network.metadata.uuid || network.status.id || network.spec.name;
}

/** The docker networks of one of one's Docker VMs, and what the session may do with them. */
export function NetworksPage() {
  const {networks: may} = useAbilities();
  const [vm, setVM] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [internal, setInternal] = useState(false);
  const [inspected, setInspected] = useState<DockerNetwork | null>(null);
  const [deleted, setDeleted] = useState<DockerNetwork | null>(null);
  const networks = useQuery(`networks:${vm}`, () => Workloads.Networks(vm ?? "") as Promise<Items<DockerNetwork>>, {interval: POLL, enabled: vm !== null});
  const creating = useRun(async () => {
    await Workloads.CreateNetwork(vm ?? "", {name: name.trim(), internal});
    setName("");
    networks.reload();
  });
  const laidOut = formFailure(creating.failure, (field) => (field === "name" || field === "driver" ? "name" : undefined));
  const items = networks.data?.items ?? [];

  const create = (event: FormEvent) => {
    event.preventDefault();
    void creating.run();
  };

  return (
    <>
      <PageHeader title="Networks" description="How the containers of a Docker VM reach one another." />
      <DockerVMSelect value={vm} onChange={setVM} />
      {vm !== null && (
        <>
          {may.create && (
            <Paper withBorder p="md" mb="md" component="form" onSubmit={create}>
              <Group align="center">
                <TextInput placeholder="backend" aria-label="Network to create" value={name} onChange={(event) => setName(event.currentTarget.value)} error={laidOut.fields.name} style={{flex: 1}} />
                <Switch label="Internal" checked={internal} onChange={(event) => setInternal(event.currentTarget.checked)} />
                <Button type="submit" leftSection={<IconPlus size={16} />} loading={creating.pending}>
                  Create
                </Button>
              </Group>
              <FailureAlert messages={laidOut.messages} failure={creating.failure} title="The network could not be created" />
            </Paper>
          )}
          <FailureAlert failure={networks.failure} title="Its networks could not be read" />
          {networks.loading && <Loader />}
          {!networks.loading && items.length === 0 && !networks.failure && (
            <EmptyState icon={<IconNetwork />} title="No networks" description={may.create ? "Create one for containers to reach one another on." : undefined} />
          )}
          {items.length > 0 && (
            <Table.ScrollContainer minWidth={640}>
              <Table verticalSpacing="sm">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Name</Table.Th>
                    <Table.Th>State</Table.Th>
                    <Table.Th>Driver</Table.Th>
                    <Table.Th>Containers</Table.Th>
                    <Table.Th>Created</Table.Th>
                    <Table.Th />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {items.map((network) => (
                    <Table.Tr key={idOf(network)}>
                      <Table.Td>
                        <Text size="sm" fw={500}>
                          {network.spec.name || network.status.name}
                        </Text>
                        <Text size="xs" c="dimmed">
                          {network.status.internal || network.spec.internal ? "internal" : ""}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <StateBadge state={network.status.state} reason={network.status.failure?.message ?? network.status.reason} />
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{network.status.driver || network.spec.driver || "bridge"}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{(network.status.containers ?? []).length}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{ago(network.status.created_at)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <WorkloadActions onInspect={() => setInspected(network)} onDelete={may.delete ? () => setDeleted(network) : undefined} />
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </>
      )}
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected?.spec.name ?? "Network"} value={inspected} />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete network"
        name={deleted?.spec.name ?? ""}
        dependents={() => Workloads.NetworkDependents(vm ?? "", deleted ? idOf(deleted) : "") as Promise<Dependents>}
        onDelete={({cascade}) => Workloads.DeleteNetwork(vm ?? "", deleted ? idOf(deleted) : "", cascade)}
        onDeleted={networks.reload}
      />
    </>
  );
}
