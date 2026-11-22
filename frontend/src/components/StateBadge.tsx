import {Badge, Tooltip} from "@mantine/core";
import {stateColor} from "../workloads/format";

/** A workload's state, coloured by what it says, with why when it says. */
export function StateBadge({state, reason}: {state?: string; reason?: string}) {
  const badge = (
    <Badge color={stateColor(state)} variant="light">
      {state || "unknown"}
    </Badge>
  );

  if (!reason) {
    return badge;
  }

  return (
    <Tooltip label={reason} multiline maw={360}>
      {badge}
    </Tooltip>
  );
}
