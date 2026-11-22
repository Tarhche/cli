import {Button, Menu, Text} from "@mantine/core";
import {IconCheck, IconChevronDown, IconServer, IconSettings} from "@tabler/icons-react";
import {useState} from "react";
import {useAct} from "../api/useAct";
import {ContextsModal} from "./ContextsModal";
import {useSession} from "./session";

/** Which context the app talks to, and the others it can talk to. */
export function ContextMenu() {
  const session = useSession();
  const act = useAct();
  const [managing, setManaging] = useState(false);
  const current = session.status?.context.name ?? session.contexts.find((c) => c.current)?.name ?? "";

  return (
    <>
      <Menu position="bottom-end" width={260} withinPortal>
        <Menu.Target>
          <Button variant="default" size="xs" leftSection={<IconServer size={14} />} rightSection={<IconChevronDown size={14} />} aria-label="Context">
            <Text span size="sm" truncate maw={160}>
              {current || "No context"}
            </Text>
          </Button>
        </Menu.Target>
        <Menu.Dropdown>
          <Menu.Label>Contexts</Menu.Label>
          {session.contexts.map((context) => (
            <Menu.Item
              key={context.name}
              leftSection={context.name === current ? <IconCheck size={14} /> : <span style={{width: 14, display: "inline-block"}} />}
              onClick={() => void act(`Talking to ${context.name}`, () => session.use(context.name))}
            >
              <Text size="sm">{context.name}</Text>
              <Text size="xs" c="dimmed" truncate>
                {context.api}
              </Text>
            </Menu.Item>
          ))}
          <Menu.Divider />
          <Menu.Item leftSection={<IconSettings size={14} />} onClick={() => setManaging(true)}>
            Manage contexts
          </Menu.Item>
        </Menu.Dropdown>
      </Menu>
      <ContextsModal opened={managing} onClose={() => setManaging(false)} />
    </>
  );
}
