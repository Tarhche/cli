import {Center, Loader, MantineProvider} from "@mantine/core";
import {ModalsProvider} from "@mantine/modals";
import {Notifications} from "@mantine/notifications";
import {LoginScreen} from "./session/LoginScreen";
import {useSession} from "./session/session";
import {SessionProvider} from "./session/SessionProvider";
import {NavigationProvider} from "./shell/NavigationProvider";
import {Shell} from "./shell/Shell";
import {theme} from "./theme";

/** The desktop app. */
export function App() {
  return (
    <MantineProvider theme={theme} defaultColorScheme="auto">
      <ModalsProvider>
        <Notifications position="bottom-right" />
        <SessionProvider>
          <Gate />
        </SessionProvider>
      </ModalsProvider>
    </MantineProvider>
  );
}

/**
 * The login whenever the context the app talks to has no session, and the
 * app once it has one: a new context or session starts it afresh.
 */
export function Gate() {
  const session = useSession();

  if (session.status === undefined && session.failure === null) {
    return (
      <Center mih="100vh">
        <Loader />
      </Center>
    );
  }

  if (!session.status?.logged_in) {
    return <LoginScreen />;
  }

  return (
    <NavigationProvider key={`${session.status.context.name}/${session.status.subject ?? ""}`}>
      <Shell />
    </NavigationProvider>
  );
}
