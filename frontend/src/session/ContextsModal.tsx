import {Badge, Button, Group, Modal, Stack, Table, Text, TextInput, Title} from "@mantine/core";
import {type FormEvent, useState} from "react";
import {formFailure} from "../api/failure";
import {Session} from "../api/services";
import {useAct} from "../api/useAct";
import {useRun} from "../api/useRun";
import {FailureAlert} from "../components/FailureAlert";
import {useSession} from "./session";

/** The fields of a context, which what is refused of it is said beside. */
const CONTEXT_FIELDS = ["name", "description", "api", "ingress"];

const blank = {name: "", description: "", api: "https://", ingress: ""};

/** Lists the contexts the app can talk to, uses, creates and removes them. */
export function ContextsModal({opened, onClose}: {opened: boolean; onClose: () => void}) {
  const session = useSession();
  const act = useAct();
  const [created, setCreated] = useState(blank);
  const creation = useRun(async () => {
    await Session.Create(created);
    setCreated(blank);
    await session.refresh();
  });
  const laidOut = formFailure(creation.failure, (field) => (CONTEXT_FIELDS.includes(field) ? field : undefined));

  const create = (event: FormEvent) => {
    event.preventDefault();
    void creation.run();
  };

  const remove = (name: string) =>
    act(`Removing ${name}`, async () => {
      await Session.Remove(name);
      await session.refresh();
    });

  return (
    <Modal opened={opened} onClose={onClose} title="Contexts" size="xl">
      <Stack>
        <Text size="sm" c="dimmed">
          A context is an estate of Tarhche the app and the commands talk to: its API, and its workload ingress.
        </Text>
        <Table.ScrollContainer minWidth={560}>
          <Table verticalSpacing="xs">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>API</Table.Th>
                <Table.Th>Ingress</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {session.contexts.map((context) => (
                <Table.Tr key={context.name}>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <Text size="sm" fw={500}>
                        {context.name}
                      </Text>
                      {context.current && (
                        <Badge size="xs" variant="light">
                          current
                        </Badge>
                      )}
                    </Group>
                    {context.description && (
                      <Text size="xs" c="dimmed">
                        {context.description}
                      </Text>
                    )}
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{context.api}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{context.ingress}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" justify="flex-end" wrap="nowrap">
                      <Button size="xs" variant="light" disabled={context.current} onClick={() => void act(`Talking to ${context.name}`, () => session.use(context.name))}>
                        Use
                      </Button>
                      <Button size="xs" variant="subtle" color="red" disabled={context.name === "default"} onClick={() => void remove(context.name)}>
                        Remove
                      </Button>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
        <form onSubmit={create}>
          <Stack gap="sm">
            <Title order={4}>New context</Title>
            <FailureAlert messages={laidOut.messages} failure={creation.failure} title="It could not be created" />
            <Group grow align="flex-start">
              <TextInput label="Name" value={created.name} onChange={(e) => setCreated({...created, name: e.currentTarget.value})} error={laidOut.fields.name} />
              <TextInput label="Description" value={created.description} onChange={(e) => setCreated({...created, description: e.currentTarget.value})} error={laidOut.fields.description} />
            </Group>
            <Group grow align="flex-start">
              <TextInput label="API" value={created.api} onChange={(e) => setCreated({...created, api: e.currentTarget.value})} error={laidOut.fields.api} />
              <TextInput
                label="Workload ingress"
                placeholder="https://workload.example.com"
                value={created.ingress}
                onChange={(e) => setCreated({...created, ingress: e.currentTarget.value})}
                error={laidOut.fields.ingress}
              />
            </Group>
            <Group justify="flex-end">
              <Button type="submit" loading={creation.pending}>
                Create
              </Button>
            </Group>
          </Stack>
        </form>
      </Stack>
    </Modal>
  );
}
