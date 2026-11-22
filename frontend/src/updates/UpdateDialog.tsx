import {Alert, Anchor, Button, Group, Loader, Modal, Progress, ScrollArea, Stack, Text} from "@mantine/core";
import {IconCircleCheck, IconInfoCircle} from "@tabler/icons-react";
import {useCallback, useEffect, useRef, useState} from "react";
import {openInBrowser} from "../api/browser";
import {onUpdateProgress} from "../api/events";
import {type Update, type UpdateProgress, type Upgraded, Updates} from "../api/services";
import {useQuery} from "../api/useQuery";
import {useRun} from "../api/useRun";
import {FailureAlert} from "../components/FailureAlert";
import {ago, bytes} from "../workloads/format";
import {ReleaseNotes} from "./ReleaseNotes";

type Props = {
  opened: boolean;
  onClose: () => void;
};

/** A call the bindings answer, which may be cancelled. */
type Cancellable = {cancel?: () => void};

/**
 * Looks for a newer release of tarhche and upgrades the app to it when
 * asked: what the release says of itself, its download, and the restart
 * into it. It cannot be closed while it downloads or restarts.
 */
export function UpdateDialog({opened, onClose}: Props) {
  const [busy, setBusy] = useState(false);

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Check for updates"
      size="lg"
      centered
      withCloseButton={!busy}
      closeButtonProps={{"aria-label": "Close the dialog"}}
      closeOnClickOutside={!busy}
      closeOnEscape={!busy}
    >
      {opened && <UpdateFlow onClose={onClose} onBusy={setBusy} />}
    </Modal>
  );
}

/** Reports whether version is a release's, rather than a build of one's own (dev). */
function released(version: string): boolean {
  return /^\d+\.\d+\.\d+/.test(version);
}

/** Looks for the latest release. */
function lookForUpdates(): Promise<Update> {
  return Updates.Check();
}

/** Starts the version an upgrade installed, which quits the app. */
function restartApp(): Promise<void> {
  return Updates.Restart();
}

/** What the dialog shows, from looking for a newer release to restarting into it. */
function UpdateFlow({onClose, onBusy}: {onClose: () => void; onBusy: (busy: boolean) => void}) {
  const [looks, setLooks] = useState(0);
  const [arrived, setArrived] = useState<UpdateProgress | null>(null);
  const [upgraded, setUpgraded] = useState<Upgraded | null>(null);
  const [restarting, setRestarting] = useState(false);
  const download = useRef<Cancellable | null>(null);

  const check = useQuery(`updates:check:${looks}`, lookForUpdates);
  const update = check.data;
  const restart = useRun(restartApp);
  const upgrade = useRun(
    useCallback(async (version: string) => {
      const call: Promise<Upgraded> & Cancellable = Updates.Upgrade(version);
      download.current = call;

      try {
        return await call;
      } finally {
        download.current = null;
      }
    }, []),
  );

  useEffect(() => onUpdateProgress(setArrived), []);

  const busy = upgrade.pending || restart.pending || restarting;
  useEffect(() => onBusy(busy), [busy, onBusy]);

  const install = async (version: string) => {
    setArrived(null);

    const outcome = await upgrade.run(version);
    if (outcome.done) {
      setUpgraded(outcome.value);
    }
  };

  const restartNow = async () => {
    const outcome = await restart.run();
    if (outcome.done) {
      setRestarting(true);
    }
  };

  if (upgraded) {
    return (
      <Stack>
        <Alert color="green" icon={<IconCircleCheck />} title={`Upgraded to ${upgraded.version}`}>
          Restart Tarhche to use it.
        </Alert>
        <FailureAlert failure={restart.failure} title="Tarhche could not be restarted" />
        {restarting ? (
          <Group gap="xs">
            <Loader size="sm" />
            <Text size="sm">Restarting Tarhche…</Text>
          </Group>
        ) : (
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose} disabled={restart.pending}>
              Later
            </Button>
            <Button loading={restart.pending} onClick={() => void restartNow()}>
              Restart now
            </Button>
          </Group>
        )}
      </Stack>
    );
  }

  if (upgrade.pending) {
    return <Downloading version={update?.latest.version ?? ""} arrived={arrived} onCancel={() => download.current?.cancel?.()} />;
  }

  if (update === undefined) {
    if (check.failure) {
      return (
        <Stack>
          <FailureAlert failure={check.failure} title="Newer versions could not be looked for" />
          <Group justify="flex-end">
            <Button variant="default" onClick={onClose}>
              Close
            </Button>
            <Button onClick={() => setLooks((looked) => looked + 1)}>Try again</Button>
          </Group>
        </Stack>
      );
    }

    return (
      <Group gap="xs">
        <Loader size="sm" />
        <Text size="sm">Looking for a newer version…</Text>
      </Group>
    );
  }

  if (!update.available) {
    return (
      <Stack>
        <Text>Tarhche {update.current} is the latest version.</Text>
        <Group justify="flex-end">
          <Button onClick={onClose}>Close</Button>
        </Group>
      </Stack>
    );
  }

  const latest = update.latest;

  return (
    <Stack>
      <Stack gap={4}>
        <Text fw={600}>
          Tarhche {latest.version} is available — you have {released(update.current) ? update.current : "a development build"}.
        </Text>
        <Text size="sm" c="dimmed">
          Released {ago(latest.published_at)} ·{" "}
          <Anchor href={latest.url} inherit onClick={(event) => openInBrowser(event, latest.url)}>
            the release on GitHub
          </Anchor>
        </Text>
      </Stack>
      <ScrollArea.Autosize mah={320} type="auto" offsetScrollbars>
        <ReleaseNotes notes={latest.notes} />
      </ScrollArea.Autosize>
      {update.unsupported ? (
        <Alert color="yellow" icon={<IconInfoCircle />} title="This build cannot upgrade itself">
          {update.unsupported}
        </Alert>
      ) : (
        <Text size="xs" c="dimmed">
          Upgrading replaces {update.target}.
        </Text>
      )}
      <FailureAlert failure={upgrade.failure} title="Tarhche could not be upgraded" />
      <Group justify="flex-end">
        <Button variant="default" onClick={onClose}>
          Later
        </Button>
        {!update.unsupported && <Button onClick={() => void install(latest.version)}>Upgrade</Button>}
      </Group>
    </Stack>
  );
}

/** How much of the release being downloaded arrived, of how much there is when that is known. */
function Downloading({version, arrived, onCancel}: {version: string; arrived: UpdateProgress | null; onCancel: () => void}) {
  const done = arrived?.done ?? 0;
  const total = arrived?.total ?? 0;
  const known = total > 0;

  return (
    <Stack>
      <Text>Downloading Tarhche {version}…</Text>
      <Progress value={known ? Math.min(100, (done / total) * 100) : 100} animated={!known} striped={!known} aria-label="Download" />
      <Text size="xs" c="dimmed">
        {known ? `${bytes(done)} of ${bytes(total)}` : `${bytes(done)} so far`}
      </Text>
      <Group justify="flex-end">
        <Button variant="default" onClick={onCancel}>
          Cancel
        </Button>
      </Group>
    </Stack>
  );
}
