import {AppShell, Burger, Button, EmptyState, Group, Text, ThemeIcon} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {IconBox, IconLock} from "@tabler/icons-react";
import {type Access, useAccess} from "../access/access";
import {ResourcePage} from "../resources/ResourcePage";
import {ContextMenu} from "../session/ContextMenu";
import {UserMenu} from "../session/UserMenu";
import {ContainerPage} from "../workloads/containers/ContainerPage";
import {ContainersPage} from "../workloads/containers/ContainersPage";
import {RunContainerPage} from "../workloads/containers/RunContainerPage";
import {ImagesPage} from "../workloads/docker/ImagesPage";
import {NetworksPage} from "../workloads/docker/NetworksPage";
import {VolumesPage} from "../workloads/docker/VolumesPage";
import {ForwardsPage} from "../workloads/forwards/ForwardsPage";
import {SnapshotsPage} from "../workloads/snapshots/SnapshotsPage";
import {DeployStackPage} from "../workloads/stacks/DeployStackPage";
import {StacksPage} from "../workloads/stacks/StacksPage";
import {CreateVMPage} from "../workloads/vms/CreateVMPage";
import {VMPage} from "../workloads/vms/VMPage";
import {VMsPage} from "../workloads/vms/VMsPage";
import type {View} from "./navigation";
import {allows, homeOf, useNavigation} from "./navigation";
import {Navbar} from "./Navbar";

/**
 * The app once it is signed in: its navbar, its header and the page it is
 * on, as far as the session may see them.
 */
export function Shell() {
  const navigation = useNavigation();
  const access = useAccess();
  const [opened, {toggle, close}] = useDisclosure();

  const navigate = (view: View) => {
    close();
    navigation.navigate(view);
  };

  return (
    <AppShell header={{height: 56}} navbar={{width: 240, breakpoint: "sm", collapsed: {mobile: !opened}}} padding="md">
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between" wrap="nowrap">
          <Group gap="sm" wrap="nowrap">
            <Burger opened={opened} onClick={toggle} hiddenFrom="sm" size="sm" aria-label="Navigation" />
            <ThemeIcon variant="light" size="md" radius="md">
              <IconBox size={18} />
            </ThemeIcon>
            <Text fw={700} visibleFrom="xs">
              Tarhche
            </Text>
          </Group>
          <Group gap="sm" wrap="nowrap">
            <ContextMenu />
            <UserMenu />
          </Group>
        </Group>
      </AppShell.Header>
      <AppShell.Navbar p="xs">
        <Navbar view={navigation.view} access={access} navigate={navigate} />
      </AppShell.Navbar>
      <AppShell.Main>
        <Page view={navigation.view} access={access} navigate={navigate} />
      </AppShell.Main>
    </AppShell>
  );
}

/** The page a view is, when the session may see it. */
function Page({view, access, navigate}: {view: View; access: Access; navigate: (view: View) => void}) {
  if (!allows(access, view)) {
    return <NotAllowed home={homeOf(access)} navigate={navigate} />;
  }

  switch (view.page) {
    case "containers":
      return <ContainersPage />;
    case "container":
      return <ContainerPage key={`${view.vm}/${view.id}`} vm={view.vm} id={view.id} />;
    case "run-container":
      return <RunContainerPage vm={view.vm} />;
    case "vms":
      return <VMsPage />;
    case "vm":
      return <VMPage key={view.uuid} uuid={view.uuid} />;
    case "create-vm":
      return <CreateVMPage snapshot={view.snapshot} />;
    case "images":
      return <ImagesPage />;
    case "volumes":
      return <VolumesPage />;
    case "networks":
      return <NetworksPage />;
    case "stacks":
      return <StacksPage />;
    case "deploy-stack":
      return <DeployStackPage vm={view.vm} />;
    case "snapshots":
      return <SnapshotsPage />;
    case "forwards":
      return <ForwardsPage />;
    case "resource": {
      const definition = access.definitions.find((d) => d.name === view.name);

      return definition ? <ResourcePage key={JSON.stringify(view)} definition={definition} audience={view.audience} filters={view.filters} /> : null;
    }
  }
}

/**
 * Where the session may not go, or may go no more since its roles changed:
 * what they allow is in the navbar, and the first of it a click away.
 */
function NotAllowed({home, navigate}: {home: View | null; navigate: (view: View) => void}) {
  return (
    <EmptyState
      mt="xl"
      icon={<IconLock />}
      title={home ? "Your roles do not let you see this" : "Your roles let you see nothing here yet"}
      description={home ? "What they allow is in the navigation." : "Ask an administrator for a role, and what it allows appears here as your session is renewed."}
    >
      {home && (
        <EmptyState.Actions>
          <Button variant="light" onClick={() => navigate(home)}>
            Go to what you may see
          </Button>
        </EmptyState.Actions>
      )}
    </EmptyState>
  );
}
