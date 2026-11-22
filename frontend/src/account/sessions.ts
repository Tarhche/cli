/** What a session was signed in with, as the API tells it. */
export type SessionDevice = {
  user_agent?: string;
  fingerprint?: string;
  browser?: string;
  browser_version?: string;
  os?: string;
  os_version?: string;

  /** "desktop", "mobile", "tablet" or "bot", when its user agent says. */
  type?: string;
  model?: string;
  language?: string;
  timezone?: string;
  screen?: string;
};

/**
 * One of the sessions one is signed in with: each time one signed in,
 * approved an application, or opened a shadow session to be seen as somebody
 * else, as the API answers it.
 */
export type LoginSession = {
  uuid: string;

  /** Whether it is the session the app is signed in with. */
  current: boolean;
  device: SessionDevice;

  /** The application it was handed to. */
  client?: {id: string; name?: string};

  /** Who a shadow session is seen as. */
  impersonated_user?: {uuid: string; name?: string; username?: string; avatar?: string};

  /** Where it was signed in from. */
  ip?: string;
  created_at: string;
  last_activity: {at: string; ip?: string};
  expires_at: string;
};

/** What a session is, as far as which icon it is shown with. */
export type SessionKind = "application" | "shadow" | "desktop" | "mobile" | "tablet" | "bot" | "unknown";

/** What each kind of device a user agent can say is called. */
const DEVICE_TYPES = {desktop: "Desktop", mobile: "Phone", tablet: "Tablet", bot: "Bot"} as const;

type DeviceType = keyof typeof DEVICE_TYPES;

function isDeviceType(type?: string): type is DeviceType {
  return type !== undefined && Object.keys(DEVICE_TYPES).includes(type);
}

/** What a session is: an application's, a shadow one, or the kind of device it runs on. */
export function sessionKind(session: LoginSession): SessionKind {
  if (session.client) {
    return "application";
  }

  if (session.impersonated_user) {
    return "shadow";
  }

  return isDeviceType(session.device.type) ? session.device.type : "unknown";
}

/**
 * What a session is called: the application it was handed to, or the browser
 * it was signed in from, by its major version.
 */
export function sessionName(session: LoginSession): string {
  if (session.client) {
    return session.client.name || "An application";
  }

  const {browser, browser_version} = session.device;
  if (!browser) {
    return "Unknown device";
  }

  const major = browser_version?.split(".")[0];

  return major ? `${browser} ${major}` : browser;
}

/** The system a session's device runs, with its version. */
export function sessionSystem(session: LoginSession): string {
  const {os, os_version} = session.device;

  return [os, os_version].filter(Boolean).join(" ");
}

/** The kind of device a session runs on, when its user agent says. */
export function deviceTypeName(session: LoginSession): string | undefined {
  const {type} = session.device;

  return isDeviceType(type) ? DEVICE_TYPES[type] : undefined;
}

/** The device a session runs on: its model, its system, the kind it is. */
export function sessionDevice(session: LoginSession): string {
  return [session.device.model, sessionSystem(session), deviceTypeName(session)].filter(Boolean).join(" · ");
}

/**
 * A session named so that it is told apart from another of the same browser:
 * its name, with the device it runs on.
 */
export function sessionTitle(session: LoginSession): string {
  const device = sessionDevice(session);
  const name = sessionName(session);

  return device ? `${name} (${device})` : name;
}

/** Who a shadow session is seen as, by the name they go by. */
export function seenAs(session: LoginSession): string | undefined {
  const user = session.impersonated_user;

  return user ? user.name || user.username || user.uuid : undefined;
}
