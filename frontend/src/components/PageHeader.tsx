import {ActionIcon, Group, Stack, Text, Title, Tooltip} from "@mantine/core";
import {IconArrowLeft} from "@tabler/icons-react";
import type {ReactNode} from "react";

type Props = {
  title: ReactNode;
  description?: ReactNode;

  /** What the page offers to do, beside its title. */
  actions?: ReactNode;

  /** Goes back, when the page has somewhere to go back to. */
  onBack?: () => void;
};

/** A page's title, what it is about, and what it offers to do. */
export function PageHeader({title, description, actions, onBack}: Props) {
  return (
    <Group justify="space-between" align="flex-start" mb="md" gap="sm">
      <Group gap="xs" align="flex-start" wrap="nowrap" style={{minWidth: 0}}>
        {onBack && (
          <Tooltip label="Back">
            <ActionIcon variant="subtle" onClick={onBack} aria-label="Back" mt={4}>
              <IconArrowLeft size={18} />
            </ActionIcon>
          </Tooltip>
        )}
        <Stack gap={2} style={{minWidth: 0}}>
          <Title order={2} style={{overflowWrap: "anywhere"}}>
            {title}
          </Title>
          {description && (
            <Text c="dimmed" size="sm">
              {description}
            </Text>
          )}
        </Stack>
      </Group>
      {actions && <Group gap="xs">{actions}</Group>}
    </Group>
  );
}
