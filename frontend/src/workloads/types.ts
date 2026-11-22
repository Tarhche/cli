/**
 * The workload's resources as the API answers them: each in its kind's
 * manifest shape, {kind, metadata, spec, status}, with the control plane's
 * own field names, and beside it what the blog knows of it.
 */

/** What every resource has, whatever its kind. */
export type Metadata = {
  uuid?: string;
  name?: string;
  slug?: string;
  owner_uuid?: string;
  labels?: Record<string, string>;
  node?: string;

  /** How long it is kept, in nanoseconds; none keeps it until it is deleted. */
  lifetime?: number;
  expires_at?: string;
  deletion_requested_at?: string;
  created_at?: string;
  updated_at?: string;
};

/** What its last command failed with. */
export type CommandFailure = {
  code: string;
  message: string;
};

/** Where a resource is, in its kind's words. */
export type ResourceStatus = {
  state: string;
  expected?: string;
  reason?: string;
  since?: string;
  observed_at?: string;
  failure?: CommandFailure;
};

export type Manifest<Spec, Status> = {
  kind: string;
  metadata: Metadata;
  spec: Spec;
  status: Status;
};

/** Whole vCPUs, and memory and disk in bytes. */
export type Resources = {
  cpus: number;
  memory: number;
  disk: number;
};

/** Whether a VM's ports are reached, and whether it reaches out. */
export type Network = {
  ingress: string;
  egress: string;
};

/** The last sample of what a VM uses. */
export type VMStats = {
  cpu_percent: number;
  memory_used: number;
  memory_limit: number;
  disk_used: number;
  disk_total: number;
  network_rx: number;
  network_tx: number;
  sampled_at: string;
};

export type VMSpec = {
  image?: string;
  resources: Resources;
  ports: number[] | null;
  network: Network;
  persistent_disk?: boolean;
  source?: {snapshot: string};
};

export type VMStatus = ResourceStatus & {
  stats: VMStats | null;
  started_at?: string;
};

/** One of a VM's ports, and where it is served. */
export type VMURL = {
  port: number;
  url: string;
};

export type VM = Manifest<VMSpec, VMStatus> & {
  /** Whether it runs a dockerd, which containers and stacks run in. */
  docker: boolean;
  urls: VMURL[] | null;
};

export type PortBinding = {
  container_port: number;
  host_port: number;
  host_ip?: string;
  protocol?: string;
};

export type Mount = {
  type: string;
  source?: string;
  target: string;
  read_only?: boolean;
};

/** The sample of what a container uses. */
export type ContainerStats = {
  cpu_percent: number;
  memory_used: number;
  memory_limit: number;
  network_rx: number;
  network_tx: number;
  block_read: number;
  block_write: number;
  pids: number;
  sampled_at: string;
};

export type ContainerSpec = {
  vm: {uuid?: string; name?: string};
  name?: string;
  image: string;
  command?: string[];
  entrypoint?: string[];
  working_dir?: string;
  ports?: PortBinding[];
  mounts?: Mount[];
  networks?: string[];
  restart_policy?: string;
  cpus?: number;
  memory?: number;
  privileged?: boolean;
  user?: string;
};

/** A container's status, with what its VM's dockerd last said of it. */
export type ContainerStatus = ResourceStatus & {
  id?: string;
  name?: string;
  image?: string;
  docker_state?: string;
  docker_status?: string;
  command?: string;
  ports?: PortBinding[] | null;
  networks?: string[] | null;
  mounts?: Mount[] | null;
  restart_policy?: string;
  created_at?: string;
  stats: ContainerStats | null;
};

export type Container = Manifest<ContainerSpec, ContainerStatus> & {
  vm_uuid: string;
  vm_name: string;
};

export type Image = Manifest<
  {reference: string},
  ResourceStatus & {
    reference?: string;
    id?: string;
    tags?: string[] | null;
    size?: number;
    created_at?: string;
    in_use?: boolean;
  }
>;

export type DockerNetwork = Manifest<
  {name: string; driver?: string; internal?: boolean},
  ResourceStatus & {
    id?: string;
    name?: string;
    driver?: string;
    scope?: string;
    internal?: boolean;
    containers?: string[] | null;
    created_at?: string;
  }
>;

export type Volume = Manifest<
  {name: string; driver?: string},
  ResourceStatus & {
    name?: string;
    driver?: string;
    mountpoint?: string;
    in_use?: boolean;
    created_at?: string;
  }
>;

/** One service of a stack, and the containers made of it. */
export type StackService = {
  name: string;
  state: string;
  containers?: {id: string; name: string; state: string}[];
};

export type Stack = Manifest<
  {vm: {uuid?: string; name?: string}; compose: string},
  ResourceStatus & {services?: StackService[]}
> & {
  vm_name: string;
};

export type Snapshot = Manifest<
  {vm: {uuid: string; name?: string}},
  ResourceStatus & {
    image?: string;
    disk?: number;
    engine?: string;
    size?: number;
    completed_at?: string;
  }
> & {
  docker: boolean;
};

/** A page of a listing. */
export type Page<T> = {
  items: T[] | null;
  pagination?: {total_pages: number; current_page: number};
};

/** A listing that is not paged. */
export type Items<T> = {
  items: T[] | null;
};

/** A line a VM or a container wrote. */
export type LogLine = {
  at: string;

  /** Where it came from: a VM's kernel, main or exec. */
  source?: string;

  /** Where it went: a container's stdout or stderr. */
  stream?: string;
  line: string;
};

export type Logs = {
  items: LogLine[] | null;

  /** Whether more was written than one answer carries. */
  truncated: boolean;
};

/** What was used over a step, starting at at. */
export type UsagePoint = {
  at: string;
  cpu_percent: number;
  memory_used: number;
  memory_limit: number;
  disk_used?: number;
  disk_total?: number;
  network_rx: number;
  network_tx: number;
};

export type Usage = {
  from: string;
  to: string;
  step: number;
  points: UsagePoint[] | null;
};

/** A container run, and the Docker VM it went into. */
export type RunContainerAnswer = {
  vm?: {uuid: string; name: string; created: boolean};
  container?: Container;
};

/** A stack deployed, and the Docker VM it went into. */
export type DeployStackAnswer = {
  vm?: {uuid: string; name: string; created: boolean};
  stack?: Stack;
};
