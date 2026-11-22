import {Button, Center, Divider, Group, Loader, Paper, PasswordInput, Stack, Text, TextInput, Title} from "@mantine/core";
import {IconBrowser} from "@tabler/icons-react";
import {type FormEvent, useRef, useState} from "react";
import {formFailure} from "../api/failure";
import {Session} from "../api/services";
import {useRun} from "../api/useRun";
import {FailureAlert} from "../components/FailureAlert";
import {ContextMenu} from "./ContextMenu";
import {useSession} from "./session";

/** The fields a password login has, which what is refused is said beside. */
const LOGIN_FIELDS = ["identity", "password"];

/** A cancellable call, as the bindings answer. */
type Cancellable = Promise<void> & {cancel?: () => void};

/**
 * Signs in to the context the app talks to: in a browser, through the
 * estate's authorization server, or with an identity and a password. It is
 * what the app shows whenever that context has no session.
 */
export function LoginScreen() {
  const session = useSession();
  const [identity, setIdentity] = useState("");
  const [password, setPassword] = useState("");
  const waiting = useRef<Cancellable | null>(null);

  const browser = useRun(async () => {
    const call: Cancellable = Session.LoginWithBrowser();
    waiting.current = call;

    try {
      await call;
    } finally {
      waiting.current = null;
    }

    await session.refresh();
  });

  const passwordLogin = useRun(async () => {
    await Session.LoginWithPassword(identity, password);
    await session.refresh();
  });

  const laidOut = formFailure(passwordLogin.failure, (field) => (LOGIN_FIELDS.includes(field) ? field : undefined));
  const context = session.status?.context;

  const signIn = (event: FormEvent) => {
    event.preventDefault();
    browser.reset();
    void passwordLogin.run();
  };

  return (
    <Center mih="100vh" p="md">
      <Paper withBorder shadow="sm" p="xl" radius="md" w="100%" maw={440}>
        <Stack>
          <Group justify="space-between" align="flex-start" wrap="nowrap">
            <Stack gap={2} style={{minWidth: 0}}>
              <Title order={2}>Sign in</Title>
              <Text size="sm" c="dimmed" truncate>
                {context ? `to ${context.name}, ${context.api}` : "Reading the session…"}
              </Text>
            </Stack>
            <ContextMenu />
          </Group>

          <FailureAlert failure={session.failure} title="The session could not be read" />
          <FailureAlert failure={browser.failure} title="Signing in in the browser failed" />

          {browser.pending ? (
            <Stack align="center" gap="xs">
              <Loader size="sm" />
              <Text size="sm">Waiting for your browser…</Text>
              <Button variant="subtle" size="xs" onClick={() => waiting.current?.cancel?.()}>
                Cancel
              </Button>
            </Stack>
          ) : (
            <Button
              leftSection={<IconBrowser size={18} />}
              size="md"
              onClick={() => {
                passwordLogin.reset();
                void browser.run();
              }}
            >
              Sign in with browser
            </Button>
          )}

          <Divider label="or with a password" labelPosition="center" />

          <form onSubmit={signIn}>
            <Stack gap="sm">
              <FailureAlert messages={laidOut.messages} failure={passwordLogin.failure} title="Signing in failed" />
              <TextInput
                label="Username or email"
                autoComplete="username"
                value={identity}
                onChange={(event) => setIdentity(event.currentTarget.value)}
                error={laidOut.fields.identity}
              />
              <PasswordInput
                label="Password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.currentTarget.value)}
                error={laidOut.fields.password}
              />
              <Button type="submit" variant="light" loading={passwordLogin.pending} disabled={browser.pending}>
                Sign in
              </Button>
            </Stack>
          </form>
        </Stack>
      </Paper>
    </Center>
  );
}
