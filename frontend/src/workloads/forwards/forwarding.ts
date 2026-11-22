import type {Forward, Target} from "../../api/services";

/** How often the forwards are read again, while they are shown. */
export const FORWARDS_POLL = 3000;

/** Whether a forward reaches into target. */
export function reaches(forward: Forward, target: Target): boolean {
  return forward.target.vm === target.vm && (forward.target.container ?? "") === (target.container ?? "");
}

/** Where a forward is reached on this machine, as a URL a browser opens. */
export function urlOf(forward: Forward): string {
  return `http://${forward.address}`;
}
