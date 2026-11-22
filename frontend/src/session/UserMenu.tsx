import {Avatar, Group, Menu, Text, UnstyledButton} from "@mantine/core";
import {IconLogout} from "@tabler/icons-react";
import {Session} from "../api/services";
import {useAct} from "../api/useAct";
import {useSession} from "./session";

/** Who the app is signed in as, and signing out. */
export function UserMenu() {
  const session = useSession();
  const act = useAct();
  const status = session.status;
  const user = status?.user;
  const name = user?.name || user?.username || status?.subject || "Signed in";

  const logout = () =>
    act("Signing out", async () => {
      await Session.Logout();
      await session.refresh();
    });

  return (
    <Menu position="bottom-end" width={240} withinPortal>
      <Menu.Target>
        <UnstyledButton aria-label="Account">
          <Group gap="xs" wrap="nowrap">
            <Avatar name={name} color="initials" size="sm" radius="xl" />
            <Text size="sm" fw={500} visibleFrom="sm" truncate maw={160}>
              {name}
            </Text>
          </Group>
        </UnstyledButton>
      </Menu.Target>
      <Menu.Dropdown>
        <Menu.Label>
          {user?.username ? `@${user.username}` : name}
          {user?.email ? ` · ${user.email}` : ""}
        </Menu.Label>
        <Menu.Label>
          Signed in {status?.method === "password" ? "with a password" : "in a browser"} to {status?.context.name}
        </Menu.Label>
        <Menu.Divider />
        <Menu.Item color="red" leftSection={<IconLogout size={14} />} onClick={() => void logout()}>
          Sign out
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}
