import {Alert, List} from "@mantine/core";
import {IconAlertTriangle} from "@tabler/icons-react";
import type {Failure} from "../api/failure";

type Props = {
  /** What went wrong, nothing when nothing did. */
  failure?: Failure | null;

  /** What to say instead of the failure's message, as a form does. */
  messages?: string[];

  title?: string;
};

/** Says what went wrong, when anything did. */
export function FailureAlert({failure, messages, title = "Something went wrong"}: Props) {
  const said = messages ?? (failure ? [failure.message] : []);
  if (said.length === 0) {
    return null;
  }

  return (
    <Alert color={failure?.unavailable ? "yellow" : "red"} icon={<IconAlertTriangle />} title={title} role="alert">
      {said.length === 1 ? (
        said[0]
      ) : (
        <List size="sm">
          {said.map((message) => (
            <List.Item key={message}>{message}</List.Item>
          ))}
        </List>
      )}
    </Alert>
  );
}
