const UNITS = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

/** A size in bytes as it is read: 1.5 GiB. */
export function bytes(size: number | undefined | null): string {
  if (size === undefined || size === null || !Number.isFinite(size) || size < 0) {
    return "";
  }

  let value = size;
  let unit = 0;
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024;
    unit++;
  }

  const shown = unit === 0 || value >= 100 ? Math.round(value).toString() : value.toFixed(1).replace(/\.0$/, "");

  return `${shown} ${UNITS[unit]}`;
}

const RELATIVE = new Intl.RelativeTimeFormat("en", {numeric: "auto"});

const STEPS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 3600],
  ["month", 30 * 24 * 3600],
  ["week", 7 * 24 * 3600],
  ["day", 24 * 3600],
  ["hour", 3600],
  ["minute", 60],
  ["second", 1],
];

/** How long ago a moment was, or how long until it: 3 minutes ago. */
export function ago(moment: string | undefined, now: Date = new Date()): string {
  if (!moment) {
    return "";
  }

  const at = new Date(moment);
  if (Number.isNaN(at.getTime()) || at.getFullYear() <= 1) {
    return "";
  }

  const seconds = Math.round((at.getTime() - now.getTime()) / 1000);
  for (const [unit, size] of STEPS) {
    if (Math.abs(seconds) >= size || unit === "second") {
      return RELATIVE.format(Math.round(seconds / size), unit);
    }
  }

  return "";
}

/** A moment as it is read on this machine. */
export function moment(at: string | undefined): string {
  if (!at) {
    return "";
  }

  const date = new Date(at);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) {
    return "";
  }

  return date.toLocaleString();
}

const RUNNING = new Set(["running", "ready", "up", "healthy", "available", "pulled"]);
const DONE = new Set(["stopped", "exited", "completed", "deleted"]);
const FAILED = new Set(["failed", "dead", "error", "unhealthy"]);

/** The colour a workload's state is shown in. */
export function stateColor(state: string | undefined): string {
  if (!state) {
    return "gray";
  }

  if (RUNNING.has(state)) {
    return "green";
  }

  if (DONE.has(state)) {
    return "gray";
  }

  if (FAILED.has(state)) {
    return "red";
  }

  return "blue";
}

/** Whether something in that state can be started. */
export function canStart(state: string | undefined): boolean {
  return state === "stopped" || state === "exited" || state === "completed" || state === "failed" || state === "created";
}

/** Whether something in that state can be stopped. */
export function canStop(state: string | undefined): boolean {
  return state === "running" || state === "starting" || state === "restarting";
}

/** What a VM is given, as it is read: 1 vCPU, 512 MiB, 4 GiB. */
export function sizeOf(resources: {cpus: number; memory: number; disk: number} | undefined): string {
  if (!resources) {
    return "";
  }

  const cpus = `${resources.cpus} vCPU${resources.cpus === 1 ? "" : "s"}`;

  return [cpus, bytes(resources.memory), bytes(resources.disk)].filter(Boolean).join(" · ");
}

/** A share of something, as a percentage of its limit, 0 when it has none. */
export function share(used: number | undefined, limit: number | undefined): number {
  if (!used || !limit || limit <= 0) {
    return 0;
  }

  return Math.min(100, (used / limit) * 100);
}

/** A lifetime in nanoseconds, as it is read: 1 day, 2 hours. */
export function lifetime(nanoseconds: number | undefined): string {
  if (!nanoseconds || nanoseconds <= 0) {
    return "until it is deleted";
  }

  const seconds = Math.round(nanoseconds / 1e9);
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts = [
    days > 0 ? `${days} day${days === 1 ? "" : "s"}` : "",
    hours > 0 ? `${hours} hour${hours === 1 ? "" : "s"}` : "",
    minutes > 0 && days === 0 ? `${minutes} minute${minutes === 1 ? "" : "s"}` : "",
  ].filter(Boolean);

  return parts.length > 0 ? parts.join(", ") : `${seconds} seconds`;
}

/** A container's ports, as docker ps shows them: 8080->80/tcp. */
export function portsOf(ports: {container_port: number; host_port: number; protocol?: string}[] | null | undefined): string {
  return (ports ?? [])
    .map((p) => `${p.host_port > 0 ? `${p.host_port}->` : ""}${p.container_port}/${p.protocol || "tcp"}`)
    .join(", ");
}
