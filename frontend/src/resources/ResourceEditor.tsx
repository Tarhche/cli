import {Button, Group, JsonInput, Loader, Modal, Stack} from "@mantine/core";
import {type FormEvent, useState} from "react";
import {formFailure} from "../api/failure";
import {useQuery} from "../api/useQuery";
import {useRun} from "../api/useRun";
import {FailureAlert} from "../components/FailureAlert";
import {pretty} from "./resource";

type Props = {
  opened: boolean;
  onClose: () => void;
  title: string;

  /** Reads what is edited, as it is now; nothing for something new. */
  read?: () => Promise<unknown>;

  /** Sends what was written. */
  save: (json: string) => Promise<unknown>;
  onSaved: () => void;
};

/** Creates or changes a resource as the JSON the API takes. */
export function ResourceEditor(props: Props) {
  return (
    <Modal opened={props.opened} onClose={props.onClose} title={props.title} size="xl">
      {props.opened && <Editor {...props} />}
    </Modal>
  );
}

function Editor({onClose, read, save, onSaved}: Props) {
  const current = useQuery("editor", () => (read ? read() : Promise.resolve({})));

  if (current.loading) {
    return <Loader />;
  }

  if (current.failure) {
    return <FailureAlert failure={current.failure} title="It could not be read" />;
  }

  return <EditorForm initial={pretty(current.data)} onClose={onClose} save={save} onSaved={onSaved} />;
}

function EditorForm({initial, onClose, save, onSaved}: {initial: string; onClose: () => void; save: Props["save"]; onSaved: () => void}) {
  const [json, setJSON] = useState(initial);
  const saving = useRun(save);
  const laidOut = formFailure(saving.failure, (field) => (field === "json" ? "json" : undefined));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if ((await saving.run(json)).done) {
      onSaved();
      onClose();
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)}>
      <Stack>
        <FailureAlert messages={laidOut.messages} failure={saving.failure} title="It could not be saved" />
        <JsonInput
          label="JSON"
          value={json}
          onChange={setJSON}
          formatOnBlur
          autosize
          minRows={12}
          maxRows={28}
          validationError="It is not valid JSON"
          error={laidOut.fields.json}
          spellCheck={false}
        />
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={saving.pending}>
            Save
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
