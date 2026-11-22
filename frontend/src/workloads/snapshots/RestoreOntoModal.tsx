import {Alert, Button, Group, Loader, Modal, Select, Stack, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconAlertTriangle} from "@tabler/icons-react";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {bytes} from "../format";
import {allVMs} from "../listing";
import type {Snapshot, VM} from "../types";

type Props = {
  opened: boolean;
  onClose: () => void;
  snapshot: Snapshot;
};

/** The states a VM's disk can be replaced in. */
const RESTORABLE = new Set(["running", "stopped", "failed"]);

/** Why a VM of the snapshot's kind cannot take its disk, if it cannot. */
function unfitFor(vm: VM, snapshot: Snapshot): string | null {
  if (vm.spec.resources.disk < (snapshot.status.disk ?? 0)) {
    return "its disk is too small";
  }

  return RESTORABLE.has(vm.status.state) ? null : "cannot be restored right now";
}

/**
 * Restores a snapshot onto one of one's VMs of its kind: the VM is stopped,
 * its disk is replaced with the snapshot's, and it is started again, which
 * is said before it is done.
 */
export function RestoreOntoModal({opened, onClose, snapshot}: Props) {
  return (
    <Modal opened={opened} onClose={onClose} title={`Restore "${snapshot.metadata.name}"`} centered>
      {opened && <RestoreOntoForm snapshot={snapshot} onClose={onClose} />}
    </Modal>
  );
}

function RestoreOntoForm({snapshot, onClose}: Omit<Props, "opened">) {
  const [chosen, setChosen] = useState<string | null>(null);
  const vms = useQuery(`vms:restorable:${snapshot.docker}`, async () => (await allVMs()).filter((vm) => vm.docker === snapshot.docker));
  const restoring = useRun(() => Workloads.RestoreVM(chosen ?? "", snapshot.metadata.uuid ?? ""));
  const laidOut = formFailure(restoring.failure, (field) => (field === "snapshot_uuid" ? "vm" : undefined));
  const candidates = vms.data ?? [];
  const target = candidates.find((vm) => vm.metadata.uuid === chosen);
  const kind = snapshot.docker ? "Docker" : "machine";

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if ((await restoring.run()).done) {
      notifications.show({color: "teal", message: `Restoring "${target?.metadata.name}". It restarts once its disk is replaced.`});
      onClose();
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)}>
      <Stack>
        <FailureAlert failure={vms.failure} title="Your VMs could not be read" />
        {vms.loading && (
          <Group justify="center">
            <Loader size="sm" />
          </Group>
        )}
        {!vms.loading && !vms.failure && candidates.length === 0 && (
          <Text size="sm" c="dimmed">
            You have no {kind} VM to restore it onto. You can restore it as a new VM instead.
          </Text>
        )}
        {candidates.length > 0 && (
          <Select
            label="VM"
            description={`One of your ${kind} VMs, with a disk of at least ${bytes(snapshot.status.disk ?? 0)}.`}
            placeholder="Choose a VM"
            data={candidates.map((vm) => {
              const unfit = unfitFor(vm, snapshot);

              return {value: vm.metadata.uuid ?? "", label: unfit ? `${vm.metadata.name}: ${unfit}` : (vm.metadata.name ?? ""), disabled: unfit !== null};
            })}
            value={chosen}
            onChange={setChosen}
            error={laidOut.fields.vm}
          />
        )}
        {target && (
          <Alert variant="light" color="orange" icon={<IconAlertTriangle />} title="This replaces the VM's disk">
            Restoring "{snapshot.metadata.name}" onto "{target.metadata.name}" restarts the VM and replaces its disk with the snapshot's. Whatever is on its disk now is lost.
          </Alert>
        )}
        <FailureAlert messages={laidOut.messages} failure={restoring.failure} title="It could not be restored" />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" color="orange" disabled={!target} loading={restoring.pending}>
            Restore
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
