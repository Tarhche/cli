import {ActionIcon, Group, Tooltip} from "@mantine/core";
import {IconPlayerPlay, IconPlayerStop, IconRefresh, IconTrash} from "@tabler/icons-react";
import type {MouseEvent} from "react";
import {canStart, canStop} from "./format";

type Props = {
  state?: string;
  onStart?: () => void;
  onStop?: () => void;
  onRestart?: () => void;
  onDelete?: () => void;
};

function stopping(action: () => void) {
  return (event: MouseEvent) => {
    event.stopPropagation();
    action();
  };
}

/** What a row of a workload's listing offers to do to it. */
export function WorkloadActions({state, onStart, onStop, onRestart, onDelete}: Props) {
  return (
    <Group gap={4} justify="flex-end" wrap="nowrap">
      {onStart && canStart(state) && (
        <Tooltip label="Start">
          <ActionIcon variant="subtle" color="green" aria-label="Start" onClick={stopping(onStart)}>
            <IconPlayerPlay size={16} />
          </ActionIcon>
        </Tooltip>
      )}
      {onStop && canStop(state) && (
        <Tooltip label="Stop">
          <ActionIcon variant="subtle" color="orange" aria-label="Stop" onClick={stopping(onStop)}>
            <IconPlayerStop size={16} />
          </ActionIcon>
        </Tooltip>
      )}
      {onRestart && canStop(state) && (
        <Tooltip label="Restart">
          <ActionIcon variant="subtle" aria-label="Restart" onClick={stopping(onRestart)}>
            <IconRefresh size={16} />
          </ActionIcon>
        </Tooltip>
      )}
      {onDelete && (
        <Tooltip label="Delete">
          <ActionIcon variant="subtle" color="red" aria-label="Delete" onClick={stopping(onDelete)}>
            <IconTrash size={16} />
          </ActionIcon>
        </Tooltip>
      )}
    </Group>
  );
}
