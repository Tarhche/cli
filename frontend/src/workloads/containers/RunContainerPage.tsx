import {ActionIcon, Button, Group, NumberInput, Paper, Select, SimpleGrid, Stack, Text, Textarea, TextInput} from "@mantine/core";
import {IconPlus, IconTrash} from "@tabler/icons-react";
import {type FormEvent, useState} from "react";
import {useAbilities} from "../../access/access";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {useNavigation} from "../../shell/navigation";
import {VMChoiceInput} from "../docker/VMChoiceInput";
import type {RunContainerAnswer} from "../types";
import {type ContainerForm, containerFieldOf, containerRequestOf, defaultContainerForm, type PortRow, RESTART_POLICIES} from "./containerForm";

/** A container to run, in a Docker VM of one's own or a new one. */
export function RunContainerPage({vm}: {vm?: string}) {
  const {navigate, back} = useNavigation();
  const {containers: may} = useAbilities();
  const [form, setForm] = useState<ContainerForm>(() => defaultContainerForm(vm ?? null));
  const running = useRun(() => Workloads.RunContainer(containerRequestOf(form)) as Promise<RunContainerAnswer | null>);
  const laidOut = formFailure(running.failure, containerFieldOf);
  const update = (patch: Partial<ContainerForm>) => setForm((current) => ({...current, ...patch}));
  const setPort = (index: number, patch: Partial<PortRow>) => update({ports: form.ports.map((row, i) => (i === index ? {...row, ...patch} : row))});

  const submit = async (event: FormEvent) => {
    event.preventDefault();

    const outcome = await running.run();
    if (!outcome.done) {
      return;
    }

    const answer = outcome.value;
    const id = answer?.container?.metadata.uuid;
    const where = answer?.vm?.uuid;
    navigate(id && where && may.show ? {page: "container", vm: where, id} : {page: "containers"});
  };

  return (
    <form onSubmit={(event) => void submit(event)} noValidate>
      <PageHeader title="Run container" description="Its image is pulled first when its Docker VM does not hold it." onBack={back} />
      <Stack maw={760}>
        <FailureAlert messages={laidOut.messages} failure={running.failure} title="The container could not be run" />
        <Paper withBorder p="md">
          <Stack>
            <VMChoiceInput value={form.where} onChange={(where) => update({where})} error={laidOut.fields.vm} />
            <SimpleGrid cols={{base: 1, sm: 2}}>
              <TextInput label="Image" placeholder="nginx:latest" required value={form.image} onChange={(e) => update({image: e.currentTarget.value})} error={laidOut.fields.image} />
              <TextInput label="Name" placeholder="Docker picks one when it is left empty" value={form.name} onChange={(e) => update({name: e.currentTarget.value})} error={laidOut.fields.name} />
            </SimpleGrid>
            <TextInput
              label="Command"
              description="What it runs instead of what its image says, as a shell would split it."
              value={form.command}
              onChange={(e) => update({command: e.currentTarget.value})}
              error={laidOut.fields.command}
            />
            <Textarea
              label="Environment"
              description="One KEY=value a line."
              autosize
              minRows={2}
              styles={{input: {fontFamily: "var(--mantine-font-family-monospace)"}}}
              value={form.env}
              onChange={(e) => update({env: e.currentTarget.value})}
              error={laidOut.fields.env}
            />
          </Stack>
        </Paper>
        <Paper withBorder p="md">
          <Stack gap="xs">
            <Group justify="space-between">
              <Text fw={500} size="sm">
                Ports
              </Text>
              <Button size="xs" variant="light" leftSection={<IconPlus size={14} />} onClick={() => update({ports: [...form.ports, {container: "", host: "", protocol: "tcp"}]})}>
                Publish a port
              </Button>
            </Group>
            <Text size="xs" c="dimmed">
              A port published on the VM is reached from outside when the VM serves it through the ingress too.
            </Text>
            {form.ports.map((row, index) => (
              <Group key={index} align="flex-end" wrap="nowrap">
                <NumberInput label="Container port" min={1} max={65535} allowDecimal={false} value={row.container} onChange={(container) => setPort(index, {container})} />
                <NumberInput label="VM port" description="0 lets docker pick" min={0} max={65535} allowDecimal={false} value={row.host} onChange={(host) => setPort(index, {host})} />
                <Select label="Protocol" data={["tcp", "udp"]} value={row.protocol} onChange={(protocol) => setPort(index, {protocol: protocol ?? "tcp"})} allowDeselect={false} w={100} />
                <ActionIcon variant="subtle" color="red" aria-label="Remove the port" mb={6} onClick={() => update({ports: form.ports.filter((_, i) => i !== index)})}>
                  <IconTrash size={16} />
                </ActionIcon>
              </Group>
            ))}
            {laidOut.fields.ports && (
              <Text size="xs" c="red">
                {laidOut.fields.ports}
              </Text>
            )}
          </Stack>
        </Paper>
        <Paper withBorder p="md">
          <SimpleGrid cols={{base: 1, sm: 3}}>
            <Select label="Restarts" data={RESTART_POLICIES} value={form.restartPolicy} onChange={(restartPolicy) => update({restartPolicy: restartPolicy ?? ""})} allowDeselect={false} error={laidOut.fields.restartPolicy} />
            <NumberInput label="CPUs" description="None is the VM's" min={0} decimalScale={2} value={form.cpus} onChange={(cpus) => update({cpus})} error={laidOut.fields.cpus} />
            <NumberInput label="Memory" description="None is the VM's" suffix=" MiB" min={0} allowDecimal={false} value={form.memoryMiB} onChange={(memoryMiB) => update({memoryMiB})} error={laidOut.fields.memory} />
          </SimpleGrid>
        </Paper>
        <Group justify="flex-end">
          <Button variant="default" onClick={back}>
            Cancel
          </Button>
          <Button type="submit" loading={running.pending}>
            Run
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
