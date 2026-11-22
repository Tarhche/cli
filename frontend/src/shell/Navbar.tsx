import {NavLink, ScrollArea, Stack, Text} from "@mantine/core";
import {
  IconBox,
  IconCamera,
  IconCloudDownload,
  IconDatabase,
  IconFileText,
  IconLayersIntersect,
  IconNetwork,
  IconPlugConnected,
  IconServer2,
  type IconProps,
} from "@tabler/icons-react";
import type {ComponentType} from "react";
import type {Access} from "../access/access";
import {pageOf, sectionsOf, type View, type WorkloadPage, workloadPagesOf} from "./navigation";

const ICONS: Record<WorkloadPage, ComponentType<IconProps>> = {
  containers: IconBox,
  vms: IconServer2,
  images: IconCloudDownload,
  volumes: IconDatabase,
  networks: IconNetwork,
  stacks: IconLayersIntersect,
  snapshots: IconCamera,
  forwards: IconPlugConnected,
};

type Props = {
  view: View;
  access: Access;
  navigate: (view: View) => void;
};

/**
 * The pages of the app the session may see: the workload's, then a section
 * of resources each, a section only when it has a page.
 */
export function Navbar({view, access, navigate}: Props) {
  const active = pageOf(view);
  const workload = workloadPagesOf(access.abilities);

  return (
    <ScrollArea type="auto" h="100%">
      <Stack gap="lg" pb="md">
        {workload.length > 0 && (
          <Stack gap={2}>
            <Text size="xs" fw={700} c="dimmed" tt="uppercase" px="sm" mb={4}>
              Workloads
            </Text>
            {workload.map(({page, label}) => {
              const Icon = ICONS[page];

              return <NavLink key={page} label={label} leftSection={<Icon size={18} stroke={1.6} />} active={active === page} onClick={() => navigate({page})} />;
            })}
          </Stack>
        )}
        {sectionsOf(access.definitions).map((section) => (
          <Stack gap={2} key={section.title}>
            <Text size="xs" fw={700} c="dimmed" tt="uppercase" px="sm" mb={4}>
              {section.title}
            </Text>
            {section.definitions.map((definition) => (
              <NavLink
                key={definition.name}
                label={definition.title}
                leftSection={<IconFileText size={18} stroke={1.6} />}
                active={active === `resource:${definition.name}`}
                onClick={() => navigate({page: "resource", name: definition.name})}
              />
            ))}
          </Stack>
        ))}
      </Stack>
    </ScrollArea>
  );
}
