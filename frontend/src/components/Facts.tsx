import {Table, Text} from "@mantine/core";
import type {ReactNode} from "react";

/** A fact about something: what it is called, and what it is. */
export type Fact = [label: string, value: ReactNode];

/** Facts about something, one a row; those with nothing to say are left out. */
export function Facts({facts}: {facts: Fact[]}) {
  const said = facts.filter(([, value]) => value !== undefined && value !== null && value !== "" && value !== false);

  return (
    <Table variant="vertical" layout="fixed" withTableBorder>
      <Table.Tbody>
        {said.map(([label, value]) => (
          <Table.Tr key={label}>
            <Table.Th w={180}>{label}</Table.Th>
            <Table.Td>{typeof value === "string" || typeof value === "number" ? <Text size="sm" style={{overflowWrap: "anywhere"}}>{value}</Text> : value}</Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
