import type {Abilities, Forward, Target} from "../../api/services";

/** How often the forwards are read again, while they are shown. */
export const FORWARDS_POLL = 3000;

/** Whether a forward reaches into target. */
export function reaches(forward: Forward, target: Target): boolean {
  return forward.target.vm === target.vm && (forward.target.container ?? "") === (target.container ?? "");
}

/** What a port is forwarded of: a VM, or a container in its Docker VM. */
export type ForwardKind = "vm" | "container";

/**
 * What the session may forward a port of: the VMs it lists, a port one
 * publishes under the forward permission and a Docker VM's others under the
 * exec one, and the containers it lists, under the exec permission of their
 * Docker VM, which relays them.
 */
export function forwardKindsOf(abilities: Abilities): ForwardKind[] {
  const kinds: ForwardKind[] = [];

  if (abilities.vms.list && (abilities.vms.forward || abilities.vms.exec)) {
    kinds.push("vm");
  }

  if (abilities.containers.list && abilities.containers.exec) {
    kinds.push("container");
  }

  return kinds;
}

/** Where a forward is reached on this machine, as a URL a browser opens. */
export function urlOf(forward: Forward): string {
  return `http://${forward.address}`;
}
