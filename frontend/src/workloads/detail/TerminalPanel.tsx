import {Box, Button, Group, Paper, Stack, Text, useComputedColorScheme} from "@mantine/core";
import {IconPlugConnected} from "@tabler/icons-react";
import {FitAddon} from "@xterm/addon-fit";
import {Terminal} from "@xterm/xterm";
import {useEffect, useRef, useState} from "react";
import {onTerminalExit, onTerminalOutput} from "../../api/events";
import {type Failure, failureOf} from "../../api/failure";
import {type Target, Terminals} from "../../api/services";
import {FailureAlert} from "../../components/FailureAlert";
import {binary, fromBase64, toBase64, typed} from "../terminal/bytes";
import {exitOf} from "../terminal/exit";
import {newID} from "../terminal/id";
import {WriteQueue} from "../terminal/writeQueue";

const THEMES = {
  dark: {background: "#1a1b1e", foreground: "#e9ecef", cursor: "#e9ecef", selectionBackground: "#495057"},
  light: {background: "#ffffff", foreground: "#212529", cursor: "#212529", selectionBackground: "#ced4da"},
};

type Props = {
  target: Target;

  /** What runs on it; empty is a shell. */
  command?: string[];
};

/**
 * A terminal in a workload: a VM's shell, or a shell or a command in a
 * container. What it says arrives in events, what is typed is sent a write
 * at a time, and it is closed once it is no longer shown.
 */
export function TerminalPanel({target, command}: Props) {
  const [session, setSession] = useState(0);

  return <TerminalSession key={session} target={target} command={command} onReconnect={() => setSession((s) => s + 1)} />;
}

type State = "connecting" | "open" | "ended" | "failed";

function TerminalSession({target, command, onReconnect}: Props & {onReconnect: () => void}) {
  const element = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal | null>(null);
  const scheme = useComputedColorScheme("light");
  const [state, setState] = useState<State>("connecting");
  const [failure, setFailure] = useState<Failure | null>(null);
  const [ending, setEnding] = useState("");
  const initialScheme = useRef(scheme);
  const commandKey = JSON.stringify(command ?? []);
  const vm = target.vm;
  const container = target.container ?? "";

  useEffect(() => {
    const host = element.current;
    if (host === null) {
      return;
    }

    const xterm = new Terminal({cursorBlink: true, fontSize: 13, fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace", theme: THEMES[initialScheme.current]});
    const fit = new FitAddon();
    xterm.loadAddon(fit);
    xterm.open(host);
    fit.fit();
    terminal.current = xterm;

    const id = newID();
    let opened = false;
    let disposed = false;

    const queue = new WriteQueue(
      (data) => Terminals.Write(id, toBase64(data)),
      (error) => setFailure(failureOf(error)),
    );

    const stopOutput = onTerminalOutput(id, (data) => xterm.write(fromBase64(data)));
    const stopExit = onTerminalExit(id, (exit) => {
      opened = false;
      setEnding(exitOf(exit));
      setState("ended");
    });

    const input = xterm.onData((data) => {
      if (opened) {
        queue.push(typed(data));
      }
    });
    const binaryInput = xterm.onBinary((data) => {
      if (opened) {
        queue.push(binary(data));
      }
    });
    const resized = xterm.onResize(({rows, cols}) => {
      if (opened) {
        Terminals.Resize(id, {rows, cols}).catch(() => undefined);
      }
    });

    const observer = new ResizeObserver(() => {
      if (host.clientWidth > 0 && host.clientHeight > 0) {
        fit.fit();
      }
    });
    observer.observe(host);

    const request = {target: {vm, container}, command: JSON.parse(commandKey) as string[], size: {rows: xterm.rows, cols: xterm.cols}};

    Terminals.Open(id, request).then(
      () => {
        if (disposed) {
          void Terminals.Close(id);

          return;
        }

        opened = true;
        setState("open");
        xterm.focus();
      },
      (error: unknown) => {
        if (!disposed) {
          setFailure(failureOf(error));
          setState("failed");
        }
      },
    );

    return () => {
      disposed = true;
      observer.disconnect();
      input.dispose();
      binaryInput.dispose();
      resized.dispose();
      stopOutput();
      stopExit();

      if (opened) {
        void Terminals.Close(id);
      }

      terminal.current = null;
      xterm.dispose();
    };
  }, [vm, container, commandKey]);

  useEffect(() => {
    if (terminal.current) {
      terminal.current.options.theme = THEMES[scheme];
    }
  }, [scheme]);

  return (
    <Stack>
      <Group justify="space-between">
        <Text size="sm" c="dimmed">
          {state === "connecting" && "Connecting…"}
          {state === "open" && "Connected."}
          {state === "ended" && `The terminal ${ending}.`}
          {state === "failed" && "The terminal could not be opened."}
        </Text>
        {(state === "ended" || state === "failed") && (
          <Button size="xs" variant="light" leftSection={<IconPlugConnected size={14} />} onClick={onReconnect}>
            Reconnect
          </Button>
        )}
      </Group>
      <FailureAlert failure={failure} title="The terminal failed" />
      <Paper withBorder p={4} style={{background: THEMES[scheme].background}}>
        <Box ref={element} h={460} />
      </Paper>
    </Stack>
  );
}
