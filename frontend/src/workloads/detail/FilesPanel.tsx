import {Button, Paper, SimpleGrid, Stack, Text, TextInput, Title} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconDownload, IconUpload} from "@tabler/icons-react";
import {useState} from "react";
import {Files, type Target} from "../../api/services";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";

/**
 * Copies a file or a directory between this machine and a workload, as
 * docker cp does: where on this machine is asked in its own dialogs.
 */
export function FilesPanel({target}: {target: Target}) {
  const [into, setInto] = useState("/tmp/");
  const [from, setFrom] = useState("");

  const copyIn = useRun(async () => {
    const copied = await Files.CopyIn(target, into);
    if (copied) {
      notifications.show({color: "teal", message: `Copied ${copied} into ${into}`});
    }
  });

  const copyOut = useRun(async () => {
    const copied = await Files.CopyOut(target, from);
    if (copied) {
      notifications.show({color: "teal", message: `Copied ${from} to ${copied}`});
    }
  });

  return (
    <SimpleGrid cols={{base: 1, md: 2}}>
      <Paper withBorder p="md">
        <Stack>
          <Title order={4}>Copy in</Title>
          <Text size="sm" c="dimmed">
            Choose a file or a directory on this machine, and it is copied to this path.
          </Text>
          <TextInput label="Path in the workload" value={into} onChange={(event) => setInto(event.currentTarget.value)} />
          <FailureAlert failure={copyIn.failure} title="It could not be copied in" />
          <Button leftSection={<IconUpload size={16} />} loading={copyIn.pending} onClick={() => void copyIn.run()}>
            Choose and copy in
          </Button>
        </Stack>
      </Paper>
      <Paper withBorder p="md">
        <Stack>
          <Title order={4}>Copy out</Title>
          <Text size="sm" c="dimmed">
            Copy what is at this path to where you choose on this machine.
          </Text>
          <TextInput label="Path in the workload" placeholder="/etc/hostname" value={from} onChange={(event) => setFrom(event.currentTarget.value)} />
          <FailureAlert failure={copyOut.failure} title="It could not be copied out" />
          <Button leftSection={<IconDownload size={16} />} loading={copyOut.pending} disabled={from.trim() === ""} onClick={() => void copyOut.run()}>
            Copy out
          </Button>
        </Stack>
      </Paper>
    </SimpleGrid>
  );
}
