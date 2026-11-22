import type {VMRequest} from "../../api/services";

/** What a VM is: a machine somebody opens a terminal in, or a Docker VM. */
export type VMKind = "machine" | "docker";

const MiB = 1024 * 1024;
const GiB = 1024 * MiB;

/** The images a machine is offered; anything else is typed in. */
export const IMAGES = ["ubuntu:24.04", "debian:12", "alpine:3.20"];

/** How long a VM may be kept, in seconds; zero keeps it until it is deleted. */
export const LIFETIMES = [
  {value: "0", label: "Until it is deleted"},
  {value: "3600", label: "An hour"},
  {value: "28800", label: "Eight hours"},
  {value: "86400", label: "A day"},
  {value: "604800", label: "A week"},
  {value: "2592000", label: "Thirty days"},
];

/** What the VM form holds. */
export type VMForm = {
  name: string;
  kind: VMKind;
  image: string;
  cpus: number;
  memoryMiB: number;
  diskGiB: number;
  ports: string[];
  ingress: boolean;
  egress: boolean;
  persistentDisk: boolean;
  lifetimeSeconds: string;

  /** The snapshot it is made from, if it is. */
  snapshot: string | null;
};

/** The fields of the form, which what the API refuses is said beside. */
export type VMField = "name" | "kind" | "image" | "cpus" | "memory" | "disk" | "ports" | "ingress" | "egress" | "lifetime" | "snapshot";

type KindDefaults = Pick<VMForm, "cpus" | "memoryMiB" | "diskGiB" | "ports">;

/**
 * What a VM of each kind starts with: the least the workload gives one,
 * which is more for a Docker VM, for dockerd and what it pulls, and the
 * ports a Docker VM made for a container is given.
 */
export const KIND_DEFAULTS: Record<VMKind, KindDefaults> = {
  machine: {cpus: 1, memoryMiB: 128, diskGiB: 1, ports: []},
  docker: {cpus: 1, memoryMiB: 512, diskGiB: 4, ports: ["80", "443", "8080"]},
};

/** The form a new VM starts as, from a snapshot when one is given. */
export function defaultVMForm(snapshot: string | null = null): VMForm {
  return {
    name: "",
    kind: "machine",
    image: IMAGES[0],
    ...KIND_DEFAULTS.machine,
    ports: [...KIND_DEFAULTS.machine.ports],
    ingress: true,
    egress: true,
    persistentDisk: true,
    lifetimeSeconds: "0",
    snapshot,
  };
}

/**
 * The form for another kind: what was left as the first kind starts takes
 * what the second starts with, and what somebody changed stays.
 */
export function switchKind(form: VMForm, kind: VMKind): VMForm {
  if (form.kind === kind) {
    return form;
  }

  const was = KIND_DEFAULTS[form.kind];
  const next = KIND_DEFAULTS[kind];
  const untouched = <K extends keyof KindDefaults>(field: K) => JSON.stringify(form[field]) === JSON.stringify(was[field]);

  return {
    ...form,
    kind,
    cpus: untouched("cpus") ? next.cpus : form.cpus,
    memoryMiB: untouched("memoryMiB") ? next.memoryMiB : form.memoryMiB,
    diskGiB: untouched("diskGiB") ? next.diskGiB : form.diskGiB,
    ports: untouched("ports") ? [...next.ports] : form.ports,
  };
}

/** Whether a tag of the ports input is a port, rather than anything typed. */
export function isPort(tag: string): boolean {
  return /^\d{1,5}$/.test(tag.trim());
}

/** The VM the form asks for, as the API takes it. */
export function vmRequestOf(form: VMForm): VMRequest {
  return {
    name: form.name.trim(),
    kind: form.kind,
    image: form.kind === "machine" ? form.image.trim() : "",
    resources: {cpus: form.cpus, memory: Math.round(form.memoryMiB * MiB), disk: Math.round(form.diskGiB * GiB)},
    ports: form.ports.filter(isPort).map(Number),
    network: {ingress: form.ingress ? "allow" : "deny", egress: form.egress ? "allow" : "deny"},
    persistent_disk: form.persistentDisk,
    lifetime_seconds: Number(form.lifetimeSeconds),
    snapshot_uuid: form.snapshot ?? "",
  };
}

const FIELDS: [string, VMField][] = [
  ["name", "name"],
  ["kind", "kind"],
  ["image", "image"],
  ["resources.cpus", "cpus"],
  ["resources.memory", "memory"],
  ["resources.disk", "disk"],
  ["resources", "memory"],
  ["ports", "ports"],
  ["network.ingress", "ingress"],
  ["network.egress", "egress"],
  ["network", "ingress"],
  ["lifetime_seconds", "lifetime"],
  ["snapshot_uuid", "snapshot"],
  ["snapshot", "snapshot"],
];

/** The field of the form what the API refused of its field is said beside. */
export function vmFieldOf(field: string): VMField | undefined {
  return FIELDS.find(([path]) => field === path || field.startsWith(`${path}.`))?.[1];
}
