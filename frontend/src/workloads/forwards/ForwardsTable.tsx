import {ActionIcon, Anchor, Table, Text, Tooltip} from "@mantine/core";
import {IconPlayerStop} from "@tabler/icons-react";
import {Browser} from "@wailsio/runtime";
import type {Forward} from "../../api/services";
import {ago} from "../format";
import {urlOf} from "./forwarding";

type Props = {
  forwards: Forward[];
  onStop: (forward: Forward) => void;

  /** Whether to say what each reaches into, which a workload's own page need not. */
  withTarget?: boolean;
};

/** Ports being forwarded to this machine. */
export function ForwardsTable({forwards, onStop, withTarget = false}: Props) {
  return (
    <Table.ScrollContainer minWidth={withTarget ? 640 : 420}>
      <Table verticalSpacing="xs">
        <Table.Thead>
          <Table.Tr>
            <Table.Th>On this machine</Table.Th>
            {withTarget && <Table.Th>Workload</Table.Th>}
            <Table.Th>Port</Table.Th>
            {withTarget && <Table.Th>Context</Table.Th>}
            <Table.Th>Started</Table.Th>
            <Table.Th />
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {forwards.map((forward) => (
            <Table.Tr key={forward.id}>
              <Table.Td>
                <Anchor size="sm" onClick={() => void Browser.OpenURL(urlOf(forward))}>
                  {forward.address}
                </Anchor>
              </Table.Td>
              {withTarget && (
                <Table.Td>
                  <Text size="sm">{forward.target.container ? `${forward.target.container} in ${forward.target.vm}` : forward.target.vm}</Text>
                </Table.Td>
              )}
              <Table.Td>
                <Text size="sm">{forward.port}</Text>
              </Table.Td>
              {withTarget && (
                <Table.Td>
                  <Text size="sm">{forward.context}</Text>
                </Table.Td>
              )}
              <Table.Td>
                <Text size="sm">{ago(forward.started_at)}</Text>
              </Table.Td>
              <Table.Td>
                <Tooltip label="Stop">
                  <ActionIcon variant="subtle" color="red" aria-label="Stop forwarding" onClick={() => onStop(forward)}>
                    <IconPlayerStop size={16} />
                  </ActionIcon>
                </Tooltip>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Table.ScrollContainer>
  );
}
