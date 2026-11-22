import {Button, EmptyState, Group, NumberInput, Paper, SegmentedControl, Select, Stack, Title} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconPlugConnected} from "@tabler/icons-react";
import {useEffect, useState} from "react";
import {onForwardStopped} from "../../api/events";
import {Forwards, Workloads} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {allVMs} from "../listing";
import type {Container, Items} from "../types";
import {FORWARDS_POLL} from "./forwarding";
import {ForwardsTable} from "./ForwardsTable";

/** The ports forwarded to this machine, and a port to forward. */
export function ForwardsPage() {
  const act = useAct();
  const forwards = useQuery("forwards", () => Forwards.List(), {interval: FORWARDS_POLL});
  const reload = forwards.reload;
  const listed = forwards.data ?? [];

  useEffect(
    () =>
      onForwardStopped((stopped) => {
        notifications.show({color: "orange", title: "A port forward stopped", message: stopped.error || "It stopped by itself."});
        reload();
      }),
    [reload],
  );

  return (
    <>
      <PageHeader title="Port forwards" description="Ports of your workloads, reached on this machine for as long as the app runs." />
      <Stack>
        <ForwardForm onStarted={reload} />
        <FailureAlert failure={forwards.failure} title="The forwards could not be read" />
        {listed.length > 0 ? (
          <ForwardsTable withTarget forwards={listed} onStop={(forward) => void act("Stopping the forward", () => Forwards.Stop(forward.id)).then(reload)} />
        ) : (
          !forwards.loading && <EmptyState icon={<IconPlugConnected />} title="Nothing is forwarded" description="Forward a port of a VM or a container to reach it here." />
        )}
      </Stack>
    </>
  );
}

/** A port of a VM or of a container to forward. */
function ForwardForm({onStarted}: {onStarted: () => void}) {
  const [kind, setKind] = useState("vm");
  const [chosen, setChosen] = useState<string | null>(null);
  const [port, setPort] = useState<number | string>("");
  const [localPort, setLocalPort] = useState<number | string>(0);
  const vms = useQuery("forward-vms", allVMs, {enabled: kind === "vm"});
  const containers = useQuery("forward-containers", () => Workloads.Containers("") as Promise<Items<Container>>, {enabled: kind === "container"});

  const targets =
    kind === "vm"
      ? (vms.data ?? []).map((vm) => ({value: vm.metadata.uuid ?? "", label: vm.metadata.name ?? "", ports: vm.spec.ports ?? []}))
      : (containers.data?.items ?? []).map((container) => ({
          value: `${container.vm_uuid}/${container.metadata.uuid || container.status.id}`,
          label: `${container.status.name || container.spec.name} in ${container.vm_name}`,
          ports: (container.spec.ports ?? []).map((p) => p.container_port),
        }));
  const target = targets.find((t) => t.value === chosen);

  const start = useRun(async () => {
    const [vm, container] = kind === "vm" ? [chosen ?? "", ""] : (chosen ?? "/").split("/");
    const started = await Forwards.Start({target: {vm, container}, port: Number(port) || 0, local_port: Number(localPort) || 0});
    notifications.show({color: "teal", message: `Port ${started.port} is forwarded to ${started.address}`});
    onStarted();
  });

  return (
    <Paper withBorder p="md">
      <Stack>
        <Title order={4}>Forward a port</Title>
        <SegmentedControl
          value={kind}
          onChange={(next) => {
            setKind(next);
            setChosen(null);
          }}
          data={[
            {value: "vm", label: "Of a VM"},
            {value: "container", label: "Of a container"},
          ]}
          w={{base: "100%", sm: 320}}
        />
        <Group align="flex-start" grow>
          <Select
            label={kind === "vm" ? "VM" : "Container"}
            placeholder="Choose one"
            data={targets.map(({value, label}) => ({value, label}))}
            value={chosen}
            onChange={(value) => {
              setChosen(value);
              setPort(targets.find((t) => t.value === value)?.ports[0] ?? "");
            }}
            searchable
          />
          <NumberInput
            label="Port in it"
            description={target && target.ports.length > 0 ? `It offers ${target.ports.join(", ")}` : undefined}
            min={1}
            max={65535}
            allowDecimal={false}
            value={port}
            onChange={setPort}
          />
          <NumberInput label="Port on this machine" description="0 takes any free one" min={0} max={65535} allowDecimal={false} value={localPort} onChange={setLocalPort} />
        </Group>
        <FailureAlert failure={start.failure} title="The port could not be forwarded" />
        <Group justify="flex-end">
          <Button leftSection={<IconPlugConnected size={16} />} loading={start.pending} disabled={chosen === null} onClick={() => void start.run()}>
            Forward
          </Button>
        </Group>
      </Stack>
    </Paper>
  );
}
