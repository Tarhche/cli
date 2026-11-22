import {Autocomplete, Button, Group, InputWrapper, NumberInput, Paper, SegmentedControl, Select, SimpleGrid, Stack, Switch, TagsInput, TextInput} from "@mantine/core";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useQuery} from "../../api/useQuery";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {useNavigation} from "../../shell/navigation";
import {bytes} from "../format";
import {allPages} from "../listing";
import type {Page, Snapshot, VM} from "../types";
import {defaultVMForm, IMAGES, isPort, LIFETIMES, switchKind, type VMForm, type VMKind, vmFieldOf, vmRequestOf} from "./vmForm";

/** A VM to create: a machine, or a Docker VM, from scratch or a snapshot. */
export function CreateVMPage({snapshot = null}: {snapshot?: string | null}) {
  const {navigate, back} = useNavigation();
  const [form, setForm] = useState<VMForm>(() => defaultVMForm(snapshot));
  const snapshots = useQuery("snapshots:ready", async () =>
    (await allPages<Snapshot>((page) => Workloads.Snapshots(page, "") as Promise<Page<Snapshot>>)).filter((s) => s.status.state === "ready"),
  );
  const creation = useRun(() => Workloads.CreateVM(vmRequestOf(form)) as Promise<VM | null>);
  const laidOut = formFailure(creation.failure, vmFieldOf);
  const update = (patch: Partial<VMForm>) => setForm((current) => ({...current, ...patch}));

  const submit = async (event: FormEvent) => {
    event.preventDefault();

    const outcome = await creation.run();
    if (outcome.done) {
      const uuid = outcome.value?.metadata.uuid;
      navigate(uuid ? {page: "vm", uuid} : {page: "vms"});
    }
  };

  const chosen = snapshots.data?.find((s) => s.metadata.uuid === form.snapshot);

  return (
    <form onSubmit={(event) => void submit(event)} noValidate>
      <PageHeader title="Create VM" description="A machine to open a terminal in, or a Docker VM to run containers and stacks in." onBack={back} />
      <Stack maw={720}>
        <FailureAlert messages={laidOut.messages} failure={creation.failure} title="The VM could not be created" />
        <Paper withBorder p="md">
          <Stack>
            <TextInput label="Name" required value={form.name} onChange={(e) => update({name: e.currentTarget.value})} error={laidOut.fields.name} />
            <Select
              label="From a snapshot"
              description="Its kind and image are the snapshot's, and its disk at least as large."
              placeholder="None"
              clearable
              data={(snapshots.data ?? []).map((s) => ({
                value: s.metadata.uuid ?? "",
                label: `${s.metadata.name} (${s.docker ? "docker" : "machine"}${s.status.disk ? `, ${bytes(s.status.disk)}` : ""})`,
              }))}
              value={form.snapshot}
              onChange={(value) => {
                const picked = snapshots.data?.find((s) => s.metadata.uuid === value);
                setForm((current) => ({...(picked ? switchKind(current, picked.docker ? "docker" : "machine") : current), snapshot: value}));
              }}
              error={laidOut.fields.snapshot}
            />
            <InputWrapper label="Kind" error={laidOut.fields.kind}>
              <SegmentedControl
                fullWidth
                disabled={chosen !== undefined}
                value={form.kind}
                onChange={(kind) => setForm((current) => switchKind(current, kind as VMKind))}
                data={[
                  {value: "machine", label: "Machine"},
                  {value: "docker", label: "Docker VM"},
                ]}
              />
            </InputWrapper>
            {form.kind === "machine" && !chosen && (
              <Autocomplete label="Image" description="An OCI image to boot." data={IMAGES} value={form.image} onChange={(image) => update({image})} error={laidOut.fields.image} />
            )}
          </Stack>
        </Paper>
        <Paper withBorder p="md">
          <SimpleGrid cols={{base: 1, sm: 3}}>
            <NumberInput label="vCPUs" min={1} allowDecimal={false} value={form.cpus} onChange={(cpus) => update({cpus: Number(cpus) || 0})} error={laidOut.fields.cpus} />
            <NumberInput label="Memory" suffix=" MiB" min={1} allowDecimal={false} value={form.memoryMiB} onChange={(memory) => update({memoryMiB: Number(memory) || 0})} error={laidOut.fields.memory} />
            <NumberInput label="Disk" suffix=" GiB" min={1} allowDecimal={false} value={form.diskGiB} onChange={(disk) => update({diskGiB: Number(disk) || 0})} error={laidOut.fields.disk} />
          </SimpleGrid>
        </Paper>
        <Paper withBorder p="md">
          <Stack>
            <TagsInput
              label="Ports"
              description="The guest ports the ingress serves; press Enter after each."
              value={form.ports}
              onChange={(ports) => update({ports: ports.filter(isPort)})}
              error={laidOut.fields.ports}
            />
            <Group>
              <Switch label="Its ports can be reached" checked={form.ingress} onChange={(e) => update({ingress: e.currentTarget.checked})} error={laidOut.fields.ingress} />
              <Switch label="It reaches the internet" checked={form.egress} onChange={(e) => update({egress: e.currentTarget.checked})} error={laidOut.fields.egress} />
              <Switch label="Its disk is kept across restarts" checked={form.persistentDisk} onChange={(e) => update({persistentDisk: e.currentTarget.checked})} />
            </Group>
            <Select
              label="Kept"
              data={LIFETIMES}
              value={form.lifetimeSeconds}
              onChange={(value) => update({lifetimeSeconds: value ?? "0"})}
              allowDeselect={false}
              error={laidOut.fields.lifetime}
            />
          </Stack>
        </Paper>
        <Group justify="flex-end">
          <Button variant="default" onClick={back}>
            Cancel
          </Button>
          <Button type="submit" loading={creation.pending}>
            Create
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
