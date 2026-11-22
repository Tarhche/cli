import {Button, Group, Modal, Stack, TextInput} from "@mantine/core";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";

type Props = {
  opened: boolean;
  onClose: () => void;
  snapshot: {uuid: string; name: string};
  onRenamed: () => void;
};

/** Renames a snapshot, which is all there is to change about one. */
export function RenameModal({opened, onClose, snapshot, onRenamed}: Props) {
  return (
    <Modal opened={opened} onClose={onClose} title="Rename snapshot" centered>
      {opened && <RenameForm snapshot={snapshot} onClose={onClose} onRenamed={onRenamed} />}
    </Modal>
  );
}

function RenameForm({snapshot, onClose, onRenamed}: Omit<Props, "opened">) {
  const [name, setName] = useState(snapshot.name);
  const renaming = useRun(() => Workloads.RenameSnapshot(snapshot.uuid, name));
  const laidOut = formFailure(renaming.failure, (field) => (field === "name" ? "name" : undefined));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if ((await renaming.run()).done) {
      onRenamed();
      onClose();
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)}>
      <Stack>
        <FailureAlert messages={laidOut.messages} failure={renaming.failure} title="It could not be renamed" />
        <TextInput label="Name" value={name} onChange={(event) => setName(event.currentTarget.value)} error={laidOut.fields.name} data-autofocus />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={renaming.pending}>
            Rename
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
