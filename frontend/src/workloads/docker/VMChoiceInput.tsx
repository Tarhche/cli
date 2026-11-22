import {InputWrapper, SegmentedControl, Select, Stack, TextInput} from "@mantine/core";
import {useQuery} from "../../api/useQuery";
import type {VMChoice} from "../containers/containerForm";
import {dockerVMs} from "../listing";

type Props = {
  value: VMChoice;
  onChange: (value: VMChoice) => void;
  error?: string;
};

/** Where a container or a stack runs: a Docker VM of one's own, or a new one. */
export function VMChoiceInput({value, onChange, error}: Props) {
  const vms = useQuery("docker-vms", dockerVMs);

  return (
    <InputWrapper label="Runs in" error={error}>
      <Stack gap="xs" mt={4}>
        <SegmentedControl
          fullWidth
          value={value.fresh ? "new" : "existing"}
          onChange={(chosen) => onChange({...value, fresh: chosen === "new"})}
          data={[
            {value: "existing", label: "A Docker VM of mine"},
            {value: "new", label: "A new Docker VM"},
          ]}
        />
        {value.fresh ? (
          <TextInput
            aria-label="Name of the new Docker VM"
            placeholder="Its name, which the workload picks when it is left empty"
            value={value.name}
            onChange={(event) => onChange({...value, name: event.currentTarget.value})}
          />
        ) : (
          <Select
            aria-label="Docker VM"
            placeholder={vms.loading ? "Reading your Docker VMs…" : (vms.data ?? []).length === 0 ? "You have no Docker VM" : "Choose one"}
            data={(vms.data ?? []).map((vm) => ({value: vm.metadata.uuid ?? "", label: `${vm.metadata.name} (${vm.status.state})`}))}
            value={value.vm}
            onChange={(vm) => onChange({...value, vm})}
            searchable
          />
        )}
      </Stack>
    </InputWrapper>
  );
}
