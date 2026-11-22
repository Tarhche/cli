import {Accordion, ActionIcon, Badge, Button, Code, CopyButton, Group, SimpleGrid, Skeleton, Stack, Text, ThemeIcon, Title, Tooltip} from "@mantine/core";
import {modals} from "@mantine/modals";
import {notifications} from "@mantine/notifications";
import {IconApps, IconCheck, IconCopy, IconDeviceDesktop, IconDeviceMobile, IconDeviceTablet, IconDevices, IconLogout, IconRobot, IconUserShare} from "@tabler/icons-react";
import {failureOf} from "../api/failure";
import {Resources} from "../api/services";
import {useAct} from "../api/useAct";
import {useQuery} from "../api/useQuery";
import {FailureAlert} from "../components/FailureAlert";
import {PageHeader} from "../components/PageHeader";
import {ago, moment} from "../workloads/format";
import {deviceTypeName, type LoginSession, seenAs, sessionDevice, sessionKind, type SessionKind, sessionName, sessionSystem, sessionTitle} from "./sessions";

const ICONS: Record<SessionKind, typeof IconDevices> = {
  application: IconApps,
  shadow: IconUserShare,
  desktop: IconDeviceDesktop,
  mobile: IconDeviceMobile,
  tablet: IconDeviceTablet,
  bot: IconRobot,
  unknown: IconDevices,
};

/** How often the sessions are read again, so that their activity is fresh. */
const REFRESH = 60_000;

/** The session resource, as the desktop's Resources service names it. */
const RESOURCE = "session";

/**
 * Signs one of one's sessions out. One that is not there any more has been
 * signed out already, which is what was asked for.
 */
async function revoke(uuid: string): Promise<void> {
  try {
    await Resources.Delete(RESOURCE, false, [uuid]);
  } catch (error) {
    if (failureOf(error).status !== 404) {
      throw error;
    }
  }
}

/**
 * One's sessions, as the web's profile lists them: this device first, then
 * every other one, each opened to see what it was signed in with and where
 * it was used from, and terminated from there; or every other one at once.
 */
export function SessionsPage() {
  const act = useAct();
  const listing = useQuery("sessions", () => Resources.List(RESOURCE, false, {page: 1, cursor: "", filters: {}}), {interval: REFRESH});
  const sessions = (listing.data?.rows ?? []).map((row) => row.item as LoginSession);
  const current = sessions.filter((session) => session.current);
  const others = sessions.filter((session) => !session.current);
  const now = new Date();

  const terminated = (done: boolean, message: string) => {
    if (done) {
      notifications.show({color: "green", message});
    }

    listing.reload();
  };

  const terminate = (session: LoginSession) =>
    modals.openConfirmModal({
      title: "Confirm action",
      centered: true,
      children: <Text size="sm">Terminate {sessionTitle(session)}? Whatever is signed in with it is signed out at its next request.</Text>,
      labels: {confirm: "Terminate session", cancel: "Cancel"},
      confirmProps: {color: "red"},
      onConfirm: async () => terminated(await act("Terminating the session", () => revoke(session.uuid)), "The session was terminated"),
    });

  const terminateOthers = () =>
    modals.openConfirmModal({
      title: "Confirm action",
      centered: true,
      children: <Text size="sm">Terminate the {others.length} other sessions? Only this device stays signed in.</Text>,
      labels: {confirm: "Terminate all other sessions", cancel: "Cancel"},
      confirmProps: {color: "red"},
      onConfirm: async () =>
        terminated(await act("Terminating all other sessions", () => Resources.Action(RESOURCE, "rm-others", false, [])), "All other sessions were terminated"),
    });

  const items = (list: LoginSession[]) => (
    <Accordion variant="separated" chevronPosition="right">
      {list.map((session) => (
        <SessionItem key={session.uuid} session={session} now={now} onTerminate={() => terminate(session)} />
      ))}
    </Accordion>
  );

  return (
    <>
      <PageHeader
        title="Sessions"
        description="The devices and applications signed in to your account. Open one to see what it was signed in with and where it was used from, and terminate any you do not recognize."
      />
      <Stack gap="lg">
        <FailureAlert failure={listing.failure} title="Your sessions could not be read" />
        {listing.failure && listing.data === undefined && (
          <Group>
            <Button variant="light" color="red" onClick={listing.reload}>
              Retry
            </Button>
          </Group>
        )}
        {listing.loading && (
          <Stack>
            <Skeleton h={28} w={160} />
            <Skeleton h={72} />
            <Skeleton h={72} />
          </Stack>
        )}
        {listing.data !== undefined && (
          <>
            {current.length > 0 && (
              <Stack gap="xs">
                <Title order={4}>This device</Title>
                {items(current)}
              </Stack>
            )}
            <Stack gap="xs">
              <Group justify="space-between" gap="xs">
                <Title order={4}>Other sessions</Title>
                {others.length > 0 && (
                  <Button variant="light" color="red" size="xs" leftSection={<IconLogout size={16} />} onClick={terminateOthers}>
                    Terminate all other sessions
                  </Button>
                )}
              </Group>
              {others.length > 0 ? (
                items(others)
              ) : (
                <Text size="sm" c="dimmed">
                  Your account is not signed in anywhere else.
                </Text>
              )}
            </Stack>
          </>
        )}
      </Stack>
    </>
  );
}

type SessionItemProps = {
  session: LoginSession;
  now: Date;

  /** Asks to sign it out, which the session the app is signed in with never is. */
  onTerminate: () => void;
};

/** A session, opened to see what it was signed in with and where it was used from. */
function SessionItem({session, now, onTerminate}: SessionItemProps) {
  const Icon = ICONS[sessionKind(session)];
  const {device} = session;
  const lastSeenFrom = session.last_activity.ip || session.ip;
  const shownAs = seenAs(session);
  const runsOn = sessionDevice(session);

  const details = [
    {label: "Application", value: session.client?.name},
    {label: "Seen as", value: shownAs},
    {label: "Browser", value: [device.browser, device.browser_version].filter(Boolean).join(" ")},
    {label: "Operating system", value: sessionSystem(session)},
    {label: "Model", value: device.model},
    {label: "Device type", value: deviceTypeName(session)},
    {label: "Signed in", value: moment(session.created_at)},
    {label: "Signed in from", value: session.ip},
    {label: "Last activity", value: moment(session.last_activity.at)},
    {label: "Last activity from", value: lastSeenFrom},
    {label: "Ends unless used before", value: moment(session.expires_at)},
    {label: "Language", value: device.language},
    {label: "Timezone", value: device.timezone},
    {label: "Screen", value: device.screen},
  ].filter((detail) => Boolean(detail.value));

  const lastActive = ago(session.last_activity.at, now);
  const activity = [lastSeenFrom, session.current ? "Active now" : lastActive && `Last active ${lastActive}`].filter(Boolean).join(" · ");

  return (
    <Accordion.Item value={session.uuid}>
      <Accordion.Control
        icon={
          <ThemeIcon variant="light" size="lg" radius="xl">
            <Icon size={20} stroke={1.5} />
          </ThemeIcon>
        }
      >
        <Stack gap={2}>
          <Group gap="xs">
            <Text fw={500}>{sessionName(session)}</Text>
            {session.current && (
              <Badge size="sm" variant="light" color="green">
                This device
              </Badge>
            )}
            {shownAs && (
              <Badge size="sm" variant="light" color="orange">
                Seen as {shownAs}
              </Badge>
            )}
          </Group>
          {runsOn && (
            <Text size="sm" c="dimmed">
              {runsOn}
            </Text>
          )}
          <Text size="xs" c="dimmed">
            {activity}
          </Text>
        </Stack>
      </Accordion.Control>
      <Accordion.Panel>
        <Stack gap="md">
          <SimpleGrid minColWidth={200} autoFlow="auto-fit" spacing="md" verticalSpacing="sm">
            {details.map((detail) => (
              <Stack key={detail.label} gap={2}>
                <Text size="xs" c="dimmed">
                  {detail.label}
                </Text>
                <Text size="sm" style={{overflowWrap: "anywhere"}}>
                  {detail.value}
                </Text>
              </Stack>
            ))}
          </SimpleGrid>

          {device.fingerprint && (
            <Stack gap={4}>
              <Text size="xs" c="dimmed">
                Device fingerprint
              </Text>
              <Group gap="xs" wrap="nowrap">
                <Code style={{overflowWrap: "anywhere"}}>{device.fingerprint}</Code>
                <CopyButton value={device.fingerprint} timeout={1500}>
                  {({copied, copy}) => (
                    <Tooltip label={copied ? "Copied" : "Copy"} withArrow>
                      <ActionIcon variant="subtle" color={copied ? "teal" : "gray"} onClick={copy} aria-label="Copy">
                        {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                      </ActionIcon>
                    </Tooltip>
                  )}
                </CopyButton>
              </Group>
            </Stack>
          )}

          {device.user_agent && (
            <Stack gap={4}>
              <Text size="xs" c="dimmed">
                User agent
              </Text>
              <Code block style={{whiteSpace: "pre-wrap", overflowWrap: "anywhere"}}>
                {device.user_agent}
              </Code>
            </Stack>
          )}

          {!session.current && (
            <Group justify="flex-end">
              <Button color="red" variant="light" leftSection={<IconLogout size={16} />} onClick={onTerminate}>
                Terminate session
              </Button>
            </Group>
          )}
        </Stack>
      </Accordion.Panel>
    </Accordion.Item>
  );
}
