import {Paper, Progress, Text} from "@mantine/core";

type Props = {
  label: string;

  /** How full it is, 0 to 100. */
  value: number;

  /** What it says of itself: 120 MiB of 512 MiB. */
  said: string;
};

/** How much of something is used. */
export function Meter({label, value, said}: Props) {
  return (
    <Paper withBorder p="sm">
      <Text size="xs" c="dimmed" tt="uppercase" fw={700}>
        {label}
      </Text>
      <Text size="lg" fw={600} my={4}>
        {said}
      </Text>
      <Progress value={value} color={value > 90 ? "red" : value > 70 ? "orange" : "indigo"} aria-label={label} />
    </Paper>
  );
}
