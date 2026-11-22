import {Badge, Button, EmptyState, Group, Loader, Paper, Table, Text, TextInput} from "@mantine/core";
import {IconCloudDownload} from "@tabler/icons-react";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {DeleteModal} from "../../components/DeleteModal";
import {FailureAlert} from "../../components/FailureAlert";
import {JsonDrawer} from "../../components/JsonDrawer";
import {PageHeader} from "../../components/PageHeader";
import {StateBadge} from "../../components/StateBadge";
import {ago, bytes} from "../format";
import {POLL} from "../listing";
import type {Image, Items} from "../types";
import {WorkloadActions} from "../WorkloadActions";
import {DockerVMSelect} from "./DockerVMSelect";

/** The images of one of one's Docker VMs. */
export function ImagesPage() {
  const [vm, setVM] = useState<string | null>(null);
  const [reference, setReference] = useState("");
  const [inspected, setInspected] = useState<Image | null>(null);
  const [deleted, setDeleted] = useState<Image | null>(null);
  const images = useQuery(`images:${vm}`, () => Workloads.Images(vm ?? "") as Promise<Items<Image>>, {interval: POLL, enabled: vm !== null});
  const pulling = useRun(async () => {
    await Workloads.PullImage(vm ?? "", reference.trim());
    setReference("");
    images.reload();
  });
  const laidOut = formFailure(pulling.failure, (field) => (field === "reference" ? "reference" : undefined));
  const items = images.data?.items ?? [];

  const pull = (event: FormEvent) => {
    event.preventDefault();
    void pulling.run();
  };

  return (
    <>
      <PageHeader title="Images" description="What the containers of a Docker VM are made from." />
      <DockerVMSelect value={vm} onChange={setVM} />
      {vm !== null && (
        <>
          <Paper withBorder p="md" mb="md" component="form" onSubmit={pull}>
            <Group align="flex-start">
              <TextInput
                placeholder="nginx:latest"
                aria-label="Image to pull"
                value={reference}
                onChange={(event) => setReference(event.currentTarget.value)}
                error={laidOut.fields.reference}
                style={{flex: 1}}
              />
              <Button type="submit" leftSection={<IconCloudDownload size={16} />} loading={pulling.pending}>
                Pull
              </Button>
            </Group>
            <FailureAlert messages={laidOut.messages} failure={pulling.failure} title="The image could not be pulled" />
          </Paper>
          <FailureAlert failure={images.failure} title="Its images could not be read" />
          {images.loading && <Loader />}
          {!images.loading && items.length === 0 && !images.failure && <EmptyState icon={<IconCloudDownload />} title="No images" description="Pull one, or run a container." />}
          {items.length > 0 && (
            <Table.ScrollContainer minWidth={640}>
              <Table verticalSpacing="sm">
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Image</Table.Th>
                    <Table.Th>State</Table.Th>
                    <Table.Th>Size</Table.Th>
                    <Table.Th>Created</Table.Th>
                    <Table.Th />
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {items.map((image) => (
                    <Table.Tr key={image.metadata.uuid ?? image.status.id}>
                      <Table.Td>
                        <Group gap="xs">
                          <Text size="sm" fw={500}>
                            {image.spec.reference || (image.status.tags ?? [])[0] || image.status.id}
                          </Text>
                          {image.status.in_use && (
                            <Badge size="xs" variant="light">
                              in use
                            </Badge>
                          )}
                        </Group>
                        <Text size="xs" c="dimmed">
                          {image.status.id?.slice(0, 19)}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <StateBadge state={image.status.state} reason={image.status.failure?.message ?? image.status.reason} />
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{bytes(image.status.size)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm">{ago(image.status.created_at)}</Text>
                      </Table.Td>
                      <Table.Td>
                        <WorkloadActions onInspect={() => setInspected(image)} onDelete={() => setDeleted(image)} />
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Table.ScrollContainer>
          )}
        </>
      )}
      <JsonDrawer opened={inspected !== null} onClose={() => setInspected(null)} title={inspected?.spec.reference ?? "Image"} value={inspected} />
      <DeleteModal
        opened={deleted !== null}
        onClose={() => setDeleted(null)}
        title="Delete image"
        name={deleted?.spec.reference || deleted?.status.id || ""}
        force="Delete it though a container uses it"
        onDelete={({force}) => Workloads.DeleteImage(vm ?? "", deleted?.metadata.uuid || deleted?.status.id || "", force)}
        onDeleted={images.reload}
      />
    </>
  );
}
