import type {ContainerRequest, NewDockerVM} from "../../api/services";

const MiB = 1024 * 1024;

/** The restart policies docker takes; empty is docker's own. */
export const RESTART_POLICIES = [
  {value: "", label: "Docker's default (no)"},
  {value: "no", label: "Never"},
  {value: "always", label: "Always"},
  {value: "unless-stopped", label: "Unless stopped"},
  {value: "on-failure", label: "On failure"},
];

/** Where something runs: a Docker VM one has, or one made for it. */
export type VMChoice = {
  /** Whether a new Docker VM is made for it. */
  fresh: boolean;

  /** The Docker VM it runs in, when it is not a new one. */
  vm: string | null;

  /** What a new Docker VM is called; empty lets the workload name it. */
  name: string;
};

/** A container port published on its VM. */
export type PortRow = {
  container: number | string;
  host: number | string;
  protocol: string;
};

/** What the container form holds. */
export type ContainerForm = {
  where: VMChoice;
  image: string;
  name: string;
  command: string;
  env: string;
  ports: PortRow[];
  restartPolicy: string;
  cpus: number | string;
  memoryMiB: number | string;
};

/** The fields of the form, which what the API refuses is said beside. */
export type ContainerField = "vm" | "image" | "name" | "command" | "env" | "ports" | "restartPolicy" | "cpus" | "memory";

/** The form a new container starts as, in vm when one is given. */
export function defaultContainerForm(vm: string | null = null): ContainerForm {
  return {
    where: {fresh: vm === null, vm, name: ""},
    image: "",
    name: "",
    command: "",
    env: "",
    ports: [],
    restartPolicy: "",
    cpus: "",
    memoryMiB: "",
  };
}

/**
 * The words of a command line, as a shell splits them: on spaces, but for
 * what is quoted, and a backslash keeps what follows it as it is.
 */
export function splitCommand(line: string): string[] {
  const words: string[] = [];
  let word = "";
  let quote: string | null = null;
  let started = false;

  for (let i = 0; i < line.length; i++) {
    const character = line[i];

    if (character === "\\" && quote !== "'" && i + 1 < line.length) {
      word += line[++i];
      started = true;
    } else if (quote !== null) {
      if (character === quote) {
        quote = null;
      } else {
        word += character;
      }
    } else if (character === "'" || character === '"') {
      quote = character;
      started = true;
    } else if (/\s/.test(character)) {
      if (started) {
        words.push(word);
        word = "";
        started = false;
      }
    } else {
      word += character;
      started = true;
    }
  }

  if (started) {
    words.push(word);
  }

  return words;
}

/** The environment a textarea holds: a KEY=value a line, blank lines aside. */
export function envOf(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

/** Where something is asked to run, as the API takes it. */
export function vmChoiceOf(where: VMChoice): {vm_uuid?: string; vm?: NewDockerVM | null} {
  if (!where.fresh) {
    return {vm_uuid: where.vm ?? ""};
  }

  return where.name.trim() ? {vm: {name: where.name.trim()}} : {};
}

/** The container the form asks for, as the API takes it. */
export function containerRequestOf(form: ContainerForm): ContainerRequest {
  return {
    ...vmChoiceOf(form.where),
    image: form.image.trim(),
    name: form.name.trim(),
    command: splitCommand(form.command),
    env: envOf(form.env),
    ports: form.ports
      .filter((row) => row.container !== "")
      .map((row) => ({container_port: Number(row.container), host_port: Number(row.host) || 0, protocol: row.protocol})),
    restart_policy: form.restartPolicy,
    cpus: Number(form.cpus) || 0,
    memory: Math.round((Number(form.memoryMiB) || 0) * MiB),
  };
}

const FIELDS: [string, ContainerField][] = [
  ["vm_uuid", "vm"],
  ["vm", "vm"],
  ["image", "image"],
  ["name", "name"],
  ["command", "command"],
  ["entrypoint", "command"],
  ["env", "env"],
  ["ports", "ports"],
  ["restart_policy", "restartPolicy"],
  ["cpus", "cpus"],
  ["memory", "memory"],
];

/** The field of the form what the API refused of its field is said beside. */
export function containerFieldOf(field: string): ContainerField | undefined {
  return FIELDS.find(([path]) => field === path || field.startsWith(`${path}.`))?.[1];
}
