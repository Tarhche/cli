import {Select} from "@mantine/core";
import {useEffect} from "react";
import {useQuery} from "../../api/useQuery";
import {FailureAlert} from "../../components/FailureAlert";
import {dockerVMs} from "../listing";

type Props = {
  value: string | null;
  onChange: (vm: string | null) => void;
};

/** Which of one's Docker VMs a page shows what is in, the first running one at first. */
export function DockerVMSelect({value, onChange}: Props) {
  const vms = useQuery("docker-vms", dockerVMs);
  const read = vms.data;
  const listed = read ?? [];

  useEffect(() => {
    if (value === null && read !== undefined && read.length > 0) {
      onChange((read.find((vm) => vm.status.state === "running") ?? read[0]).metadata.uuid ?? null);
    }
  }, [value, read, onChange]);

  return (
    <>
      <FailureAlert failure={vms.failure} title="Your Docker VMs could not be read" />
      <Select
        label="Docker VM"
        placeholder={vms.loading ? "Reading your Docker VMs…" : listed.length === 0 ? "You have no Docker VM" : "Choose one"}
        data={listed.map((vm) => ({value: vm.metadata.uuid ?? "", label: `${vm.metadata.name} (${vm.status.state})`}))}
        value={value}
        onChange={onChange}
        allowDeselect={false}
        searchable
        maw={420}
        mb="md"
      />
    </>
  );
}
