import {Text} from "@mantine/core";
import {modals} from "@mantine/modals";

/** A kind of the workload whose stop and restart interrupt what runs in it. */
export type Interruptible = "vm" | "container" | "stack";

/** What interrupts it. */
type Interruption = "stop" | "restart";

/** What a kind is called in a question's title. */
const NAMES: Record<Interruptible, string> = {
  vm: "VM",
  container: "container",
  stack: "stack",
};

/** What stopping and restarting each kind does, as the web dashboard says it. */
const QUESTIONS: Record<Interruptible, Record<Interruption, (name: string) => string>> = {
  vm: {
    stop: (name) => `Stop "${name}"? Everything running in it stops with it.`,
    restart: (name) => `Restart "${name}"? Everything running in it is stopped and started again.`,
  },
  container: {
    stop: (name) => `Stop "${name}"? Whatever it is doing is interrupted.`,
    restart: (name) => `Restart "${name}"? Whatever it is doing is interrupted.`,
  },
  stack: {
    stop: (name) => `Stop "${name}"? Every container in it is stopped.`,
    restart: (name) => `Restart "${name}"? Every container in it is restarted.`,
  },
};

const LABELS: Record<Interruption, string> = {stop: "Stop", restart: "Restart"};

function ask(kind: Interruptible, interruption: Interruption, name: string, onConfirm: () => void) {
  modals.openConfirmModal({
    title: `${LABELS[interruption]} ${NAMES[kind]}`,
    centered: true,
    children: <Text size="sm">{QUESTIONS[kind][interruption](name)}</Text>,
    labels: {confirm: LABELS[interruption], cancel: "Cancel"},
    confirmProps: interruption === "stop" ? {color: "orange"} : undefined,
    onConfirm,
  });
}

/** Asks whether to stop something, which interrupts what runs in it, and stops it once that is confirmed. */
export function confirmStop(kind: Interruptible, name: string, onConfirm: () => void) {
  ask(kind, "stop", name, onConfirm);
}

/** Asks whether to restart something, which interrupts what runs in it, and restarts it once that is confirmed. */
export function confirmRestart(kind: Interruptible, name: string, onConfirm: () => void) {
  ask(kind, "restart", name, onConfirm);
}
