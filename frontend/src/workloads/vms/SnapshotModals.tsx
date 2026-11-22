import {Button, Group, Modal, Select, Stack, Text, TextInput} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {ago} from "../format";
import type {Page, Snapshot} from "../types";

type Props = {
  opened: boolean;
  onClose: () => void;
  vm: {uuid: string; name: string};
};

/** Takes a snapshot of a VM, which has to be running or stopped. */
export function TakeSnapshotModal({opened, onClose, vm}: Props) {
  return (
    <Modal opened={opened} onClose={onClose} title={`Take a snapshot of ${vm.name}`} centered>
      {opened && <TakeSnapshotForm vm={vm} onClose={onClose} />}
    </Modal>
  );
}

function TakeSnapshotForm({vm, onClose}: {vm: Props["vm"]; onClose: () => void}) {
  const [name, setName] = useState(`${vm.name}-${new Date().toISOString().slice(0, 10)}`);
  const taking = useRun(() => Workloads.CreateSnapshot(vm.uuid, name));
  const laidOut = formFailure(taking.failure, (field) => (field === "name" ? "name" : undefined));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if ((await taking.run()).done) {
      notifications.show({color: "teal", message: `A snapshot of ${vm.name} is being taken.`});
      onClose();
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)}>
      <Stack>
        <Text size="sm" c="dimmed">
          Its disk is kept as it is now; a VM can be made from it, or restored to it.
        </Text>
        <FailureAlert messages={laidOut.messages} failure={taking.failure} title="The snapshot could not be taken" />
        <TextInput label="Name" value={name} onChange={(event) => setName(event.currentTarget.value)} error={laidOut.fields.name} data-autofocus />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={taking.pending}>
            Take
          </Button>
        </Group>
      </Stack>
    </form>
  );
}

/** Replaces a VM's disk with one of its snapshots'. */
export function RestoreSnapshotModal({opened, onClose, vm}: Props) {
  return (
    <Modal opened={opened} onClose={onClose} title={`Restore ${vm.name}`} centered>
      {opened && <RestoreForm vm={vm} onClose={onClose} />}
    </Modal>
  );
}

function RestoreForm({vm, onClose}: {vm: Props["vm"]; onClose: () => void}) {
  const [snapshot, setSnapshot] = useState<string | null>(null);
  const snapshots = useQuery(`snapshots:${vm.uuid}`, () => Workloads.Snapshots(1, vm.uuid) as Promise<Page<Snapshot>>);
  const restoring = useRun(() => Workloads.RestoreVM(vm.uuid, snapshot ?? ""));
  const laidOut = formFailure(restoring.failure, (field) => (field === "snapshot_uuid" ? "snapshot" : undefined));
  const ready = (snapshots.data?.items ?? []).filter((s) => s.status.state === "ready");

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if ((await restoring.run()).done) {
      notifications.show({color: "teal", message: `${vm.name} is being restored.`});
      onClose();
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)}>
      <Stack>
        <Text size="sm" c="dimmed">
          What is on its disk now is replaced by what the snapshot kept.
        </Text>
        <FailureAlert failure={snapshots.failure} title="Its snapshots could not be read" />
        <FailureAlert messages={laidOut.messages} failure={restoring.failure} title="It could not be restored" />
        <Select
          label="Snapshot"
          placeholder={snapshots.loading ? "Reading…" : ready.length === 0 ? "It has no snapshot ready" : "Choose one"}
          data={ready.map((s) => ({value: s.metadata.uuid ?? "", label: `${s.metadata.name}, ${ago(s.metadata.created_at)}`}))}
          value={snapshot}
          onChange={setSnapshot}
          error={laidOut.fields.snapshot}
        />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" color="orange" loading={restoring.pending}>
            Restore
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
