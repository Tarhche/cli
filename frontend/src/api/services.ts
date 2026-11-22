/**
 * The desktop's services, as Wails binds them: every call the frontend
 * makes to Go goes through here, which is also what a test stands in for.
 */
export {
  Files,
  Forwards,
  Resources,
  Session,
  Terminals,
  Workloads,
} from "../../bindings/github.com/Tarhche/cli/internal/desktop";

export type {
  ContainerRequest,
  ContextInfo,
  Forward,
  ForwardRequest,
  ForwardStopped,
  Listing,
  ListQuery,
  LogsQuery,
  NetworkRequest,
  NewContext,
  NewDockerVM,
  PortBinding,
  ResourceAction,
  ResourceDefinition,
  ResourceFilter,
  Routes,
  Row,
  StackRequest,
  Status,
  Target,
  TerminalExit,
  TerminalOutput,
  TerminalSize,
  UsageQuery,
  User,
  VMNetwork,
  VMRequest,
  VMResources,
  VolumeRequest,
} from "../../bindings/github.com/Tarhche/cli/internal/desktop/models";
