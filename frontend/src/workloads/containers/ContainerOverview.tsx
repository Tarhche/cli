import {Anchor, Code, Group} from "@mantine/core";
import {Facts} from "../../components/Facts";
import {StateBadge} from "../../components/StateBadge";
import {useNavigation} from "../../shell/navigation";
import {ago, bytes, moment, portsOf} from "../format";
import type {Container} from "../types";

/** What a container is, as it was asked for and as its VM's dockerd has it. */
export function ContainerOverview({container}: {container: Container}) {
  const {navigate} = useNavigation();
  const {spec, status} = container;

  return (
    <Facts
      facts={[
        ["Name", status.name || spec.name],
        ["UUID", container.metadata.uuid && <Code key="uuid">{container.metadata.uuid}</Code>],
        ["Docker id", status.id && <Code key="id">{status.id.slice(0, 12)}</Code>],
        ["State", <StateBadge key="state" state={status.state} reason={status.failure?.message ?? status.reason} />],
        ["Docker says", status.docker_status],
        ["Image", spec.image],
        [
          "VM",
          <Anchor key="vm" size="sm" onClick={() => navigate({page: "vm", uuid: container.vm_uuid})}>
            {container.vm_name || container.vm_uuid}
          </Anchor>,
        ],
        ["Command", status.command || [...(spec.entrypoint ?? []), ...(spec.command ?? [])].join(" ")],
        ["Working directory", spec.working_dir],
        ["Ports", portsOf(status.ports ?? spec.ports)],
        ["Networks", (status.networks ?? spec.networks ?? []).join(", ")],
        [
          "Mounts",
          (status.mounts ?? spec.mounts ?? []).length > 0 && (
            <Group gap={4} key="mounts">
              {(status.mounts ?? spec.mounts ?? []).map((mount) => (
                <Code key={mount.target}>
                  {mount.type}: {mount.source ? `${mount.source} → ` : ""}
                  {mount.target}
                  {mount.read_only ? " (read only)" : ""}
                </Code>
              ))}
            </Group>
          ),
        ],
        ["Restarts", status.restart_policy || spec.restart_policy],
        ["CPUs", spec.cpus],
        ["Memory", spec.memory ? bytes(spec.memory) : ""],
        ["User", spec.user],
        ["Privileged", spec.privileged ? "yes" : ""],
        ["Failure", status.failure && `${status.failure.code}: ${status.failure.message}`],
        ["Created", container.metadata.created_at && `${moment(container.metadata.created_at)} (${ago(container.metadata.created_at)})`],
      ]}
    />
  );
}
