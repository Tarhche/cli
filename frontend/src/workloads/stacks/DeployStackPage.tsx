import {Button, Group, Paper, Stack, Textarea, TextInput} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {type FormEvent, useState} from "react";
import {formFailure} from "../../api/failure";
import {Workloads} from "../../api/services";
import {useRun} from "../../api/useRun";
import {FailureAlert} from "../../components/FailureAlert";
import {PageHeader} from "../../components/PageHeader";
import {useNavigation} from "../../shell/navigation";
import {VMChoiceInput} from "../docker/VMChoiceInput";
import type {DeployStackAnswer} from "../types";
import {defaultStackForm, type StackForm, stackFieldOf, stackRequestOf} from "./stackForm";

/** A stack to deploy from its compose file, in a Docker VM of one's own or a new one. */
export function DeployStackPage({vm}: {vm?: string}) {
  const {navigate, back} = useNavigation();
  const [form, setForm] = useState<StackForm>(() => defaultStackForm(vm ?? null));
  const deploying = useRun(() => Workloads.DeployStack(stackRequestOf(form)) as Promise<DeployStackAnswer | null>);
  const laidOut = formFailure(deploying.failure, stackFieldOf);

  const submit = async (event: FormEvent) => {
    event.preventDefault();

    const outcome = await deploying.run();
    if (outcome.done) {
      notifications.show({color: "teal", message: `${form.name || "The stack"} is being deployed${outcome.value?.vm?.created ? ` in a new Docker VM, ${outcome.value.vm.name}` : ""}.`});
      navigate({page: "stacks"});
    }
  };

  const composeErrors = Object.entries(deploying.failure?.fields ?? {})
    .filter(([field]) => stackFieldOf(field) === "compose")
    .map(([field, message]) => (field === "compose" ? message : `${field.slice("compose.".length)}: ${message}`));

  return (
    <form onSubmit={(event) => void submit(event)} noValidate>
      <PageHeader title="Deploy stack" description="Its services, volumes and networks are made in its Docker VM, under the stack's own name." onBack={back} />
      <Stack maw={760}>
        <FailureAlert messages={laidOut.messages} failure={deploying.failure} title="The stack could not be deployed" />
        <Paper withBorder p="md">
          <Stack>
            <TextInput label="Name" required value={form.name} onChange={(e) => setForm({...form, name: e.currentTarget.value})} error={laidOut.fields.name} />
            <VMChoiceInput value={form.where} onChange={(where) => setForm({...form, where})} error={laidOut.fields.vm} />
            <Textarea
              label="Compose file"
              description="Services, volumes and networks, as a compose file says them."
              autosize
              minRows={12}
              spellCheck={false}
              styles={{input: {fontFamily: "var(--mantine-font-family-monospace)"}}}
              value={form.compose}
              onChange={(e) => setForm({...form, compose: e.currentTarget.value})}
              error={composeErrors.length > 0 ? composeErrors.join("; ") : undefined}
            />
          </Stack>
        </Paper>
        <Group justify="flex-end">
          <Button variant="default" onClick={back}>
            Cancel
          </Button>
          <Button type="submit" loading={deploying.pending}>
            Deploy
          </Button>
        </Group>
      </Stack>
    </form>
  );
}
