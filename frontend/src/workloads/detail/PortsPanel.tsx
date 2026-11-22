import {Badge, Button, Group, NumberInput, Paper, Stack, Text, Title} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconPlugConnected} from "@tabler/icons-react";
import {useEffect, useState} from "react";
import {onForwardStopped} from "../../api/events";
import {formFailure} from "../../api/failure";
import {Forwards, type Target} from "../../api/services";
import {useAct} from "../../api/useAct";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {FORWARDS_POLL, reaches} from "../forwards/forwarding";
import {ForwardsTable} from "../forwards/ForwardsTable";

type Props = {
  target: Target;

  /** The ports it offers to forward: a VM's published ports, a container's. */
  ports: number[];

  /** Says which ports can be forwarded at all. */
  hint?: string;
};

/** Forwards a workload's ports to this machine, and lists those that are. */
export function PortsPanel({target, ports, hint}: Props) {
  const act = useAct();
  const [port, setPort] = useState<number | string>(ports[0] ?? "");
  const [localPort, setLocalPort] = useState<number | string>(0);
  const forwards = useQuery(`forwards:${target.vm}/${target.container ?? ""}`, () => Forwards.List(), {interval: FORWARDS_POLL});
  const reload = forwards.reload;
  const start = useRun(async () => {
    const started = await Forwards.Start({target, port: Number(port) || 0, local_port: Number(localPort) || 0});
    notifications.show({color: "teal", message: `Port ${started.port} is forwarded to ${started.address}`});
    reload();
  });
  const laidOut = formFailure(start.failure, (field) => (field === "port" || field === "local_port" ? field : undefined));
  const listed = (forwards.data ?? []).filter((forward) => reaches(forward, target));

  useEffect(() => onForwardStopped(reload), [reload]);

  return (
    <Stack>
      <Paper withBorder p="md">
        <Stack>
          <Title order={4}>Forward a port</Title>
          {hint && (
            <Text size="sm" c="dimmed">
              {hint}
            </Text>
          )}
          {ports.length > 0 && (
            <Group gap="xs">
              {ports.map((offered) => (
                <Badge key={offered} variant={Number(port) === offered ? "filled" : "light"} style={{cursor: "pointer"}} onClick={() => setPort(offered)}>
                  {offered}
                </Badge>
              ))}
            </Group>
          )}
          <Group align="flex-start" grow>
            <NumberInput label="Port in the workload" min={1} max={65535} allowDecimal={false} value={port} onChange={setPort} error={laidOut.fields.port} />
            <NumberInput
              label="Port on this machine"
              description="0 takes any free one"
              min={0}
              max={65535}
              allowDecimal={false}
              value={localPort}
              onChange={setLocalPort}
              error={laidOut.fields.local_port}
            />
          </Group>
          <FailureAlert messages={laidOut.messages} failure={start.failure} title="The port could not be forwarded" />
          <Group justify="flex-end">
            <Button leftSection={<IconPlugConnected size={16} />} loading={start.pending} onClick={() => void start.run()}>
              Forward
            </Button>
          </Group>
        </Stack>
      </Paper>
      <FailureAlert failure={forwards.failure} title="The forwards could not be read" />
      {listed.length > 0 && (
        <ForwardsTable forwards={listed} onStop={(forward) => void act("Stopping the forward", () => Forwards.Stop(forward.id)).then(reload)} />
      )}
    </Stack>
  );
}
