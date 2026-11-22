import type {Access} from "../access/access";
import type {Abilities, KindAbilities, ResourceDefinition} from "../api/services";

/** What may be done with a kind of the workload: nothing. */
const NOTHING: KindAbilities = {
  list: false,
  show: false,
  create: false,
  start: false,
  stop: false,
  restart: false,
  update: false,
  restore: false,
  delete: false,
  logs: false,
  usage: false,
  attach: false,
  exec: false,
  forward: false,
};

/** What may be done with a kind of the workload: everything. */
const EVERYTHING: KindAbilities = {
  list: true,
  show: true,
  create: true,
  start: true,
  stop: true,
  restart: true,
  update: true,
  restore: true,
  delete: true,
  logs: true,
  usage: true,
  attach: true,
  exec: true,
  forward: true,
};

/** What a test allows of each kind of the workload, by kind. */
export type Allowed = Partial<Record<keyof Abilities, Partial<KindAbilities>>>;

/** What a session may do of the workload: nothing but what allowed says. */
export function abilities(allowed: Allowed = {}): Abilities {
  return {
    vms: {...NOTHING, ...allowed.vms},
    containers: {...NOTHING, ...allowed.containers},
    images: {...NOTHING, ...allowed.images},
    networks: {...NOTHING, ...allowed.networks},
    volumes: {...NOTHING, ...allowed.volumes},
    stacks: {...NOTHING, ...allowed.stacks},
    snapshots: {...NOTHING, ...allowed.snapshots},
  };
}

/** What a session whose roles allow everything may do of the workload. */
export function everything(): Abilities {
  return {
    vms: {...EVERYTHING},
    containers: {...EVERYTHING},
    images: {...EVERYTHING},
    networks: {...EVERYTHING},
    volumes: {...EVERYTHING},
    stacks: {...EVERYTHING},
    snapshots: {...EVERYTHING},
  };
}

/** What a session may see and do, as a test says: everything of the workload unless it says otherwise. */
export function access(state: Partial<Access> = {}): Access {
  return {definitions: [], abilities: everything(), ...state};
}

/** A resource the session may list, as a test describes it. */
export function definition(name: string, section: string, audiences: string[]): ResourceDefinition {
  return {name, section, audiences, title: name} as unknown as ResourceDefinition;
}
