import {Button, Checkbox, Group, List, Loader, Modal, Stack, Text} from "@mantine/core";
import {type ReactNode, useState} from "react";
import {useQuery} from "../api/useQuery";
import {useRun} from "../api/useRun";
import {FailureAlert} from "./FailureAlert";

/** Something that needs what is deleted. */
export type Dependent = {
  kind: string;
  uuid: string;
  name?: string;

  /** Whether it makes what is deleted again, unless it goes too. */
  recreates: boolean;
};

/** What needs something, as the API says before it is deleted. */
export type Dependents = {
  dependents: Dependent[] | null;
  recreated: boolean;
};

/** How something is deleted. */
export type Deletion = {
  /** Whether what needs it goes with it. */
  cascade: boolean;

  /** Whether it goes though it is in use. */
  force: boolean;
};

type Props = {
  opened: boolean;
  onClose: () => void;
  title: string;

  /** What is deleted, as it is shown. */
  name: string;

  description?: ReactNode;

  /** Reads what needs it, which deleting it can take along. */
  dependents?: () => Promise<Dependents>;

  /** Offers to delete it though it is in use, saying what that does. */
  force?: string;

  onDelete: (deletion: Deletion) => Promise<unknown>;
  onDeleted?: () => void;
};

/** Asks whether to delete something, and how, and deletes it. */
export function DeleteModal(props: Props) {
  return (
    <Modal opened={props.opened} onClose={props.onClose} title={props.title} centered>
      {props.opened && <DeleteForm {...props} />}
    </Modal>
  );
}

function DeleteForm({onClose, name, description, dependents, force, onDelete, onDeleted}: Props) {
  const [cascade, setCascade] = useState(false);
  const [forced, setForced] = useState(false);
  const needs = useQuery(`dependents:${name}`, () => dependents?.() ?? Promise.resolve(null), {enabled: dependents !== undefined});
  const deletion = useRun(onDelete);
  const needed = needs.data?.dependents ?? [];

  const confirm = async () => {
    const outcome = await deletion.run({cascade, force: forced});
    if (outcome.done) {
      onDeleted?.();
      onClose();
    }
  };

  return (
    <Stack>
      <Text>
        Delete <Text span fw={600}>{name}</Text>?
      </Text>
      {description && (
        <Text size="sm" c="dimmed">
          {description}
        </Text>
      )}
      {needs.loading && <Loader size="sm" />}
      <FailureAlert failure={needs.failure} title="What needs it could not be read" />
      {needed.length > 0 && (
        <Stack gap="xs">
          <Text size="sm">What needs it:</Text>
          <List size="sm">
            {needed.map((dependent) => (
              <List.Item key={dependent.uuid}>
                {dependent.kind} {dependent.name || dependent.uuid}
                {dependent.recreates ? ", which makes it again unless it goes too" : ""}
              </List.Item>
            ))}
          </List>
          <Checkbox label="Delete what needs it too" checked={cascade} onChange={(event) => setCascade(event.currentTarget.checked)} />
        </Stack>
      )}
      {force && <Checkbox label={force} checked={forced} onChange={(event) => setForced(event.currentTarget.checked)} />}
      <FailureAlert failure={deletion.failure} title="It could not be deleted" />
      <Group justify="flex-end">
        <Button variant="default" onClick={onClose}>
          Cancel
        </Button>
        <Button color="red" loading={deletion.pending} onClick={() => void confirm()}>
          Delete
        </Button>
      </Group>
    </Stack>
  );
}
