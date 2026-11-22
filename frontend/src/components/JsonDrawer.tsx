import {Code, CopyButton, Button, Drawer, Group, Loader, ScrollArea} from "@mantine/core";
import type {Failure} from "../api/failure";
import {FailureAlert} from "./FailureAlert";

type Props = {
  opened: boolean;
  onClose: () => void;
  title: string;

  /** What is inspected, as the API answered it. */
  value: unknown;

  loading?: boolean;
  failure?: Failure | null;
};

/** Shows something as the API answered it, as JSON, as inspect does. */
export function JsonDrawer({opened, onClose, title, value, loading = false, failure = null}: Props) {
  const json = JSON.stringify(value, null, 2) ?? "";

  return (
    <Drawer opened={opened} onClose={onClose} title={title} position="right" size="xl" scrollAreaComponent={ScrollArea.Autosize}>
      {loading ? (
        <Loader />
      ) : failure ? (
        <FailureAlert failure={failure} />
      ) : (
        <>
          <Group justify="flex-end" mb="xs">
            <CopyButton value={json}>
              {({copied, copy}) => (
                <Button size="xs" variant="light" onClick={copy}>
                  {copied ? "Copied" : "Copy JSON"}
                </Button>
              )}
            </CopyButton>
          </Group>
          <Code block>{json}</Code>
        </>
      )}
    </Drawer>
  );
}
