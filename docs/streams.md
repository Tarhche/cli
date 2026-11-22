# Streams

A stream reaches into a workload: a terminal, a command, or a TCP connection
to one of its ports. It is opened on the **workload ingress**, never on the
API, and carried to the node holding the workload:

```
GET {ingress}/{plural}/{uuid}/{action}[?payload=<json>]
Connection: Upgrade
Upgrade: websocket
Sec-WebSocket-Protocol: bearer, <access token>
```

- **Authentication** is the session's access token, offered as two
  subprotocols, `bearer` and the token itself; the server echoes `bearer`.
  What it may open is what the token's `permissions` claim allows (see
  [Permissions](#permissions)).
- **A payload**, for an action asked with something, is its JSON in the
  `payload` query parameter, URL-encoded. Its fields are the action's own.
- **A refusal comes before the upgrade**, as plain HTTP: `400` for a payload
  the action cannot take (`{"errors": {...}}`, or text), `401` for a missing
  or bad token, `403` for one's own workload whose action the token does not
  allow, `404` for a workload that is not there, or is somebody else's and
  the token does not allow its action on anybody's, or a stream it does not
  have, and `503` for one that is there and cannot be reached now (not
  running).
- Either side may ping; both answer pings. A client keeping a stream open
  while nothing is said (a forwarded port nobody uses) pings every 30 s, so
  that nothing on the way closes it for being idle.

## Actions

| Kind | Action | Protocol | Payload |
|------|--------|----------|---------|
| `vms` | `attach` | terminal | none: a shell, on a terminal |
| `vms` | `exec` | channel | `{"command": ["tar", "cf", "-", "/etc"], "tty": false, "rows": 24, "cols": 80, "env": ["A=b"], "working_dir": "/"}`; `command` is required and not empty, the rest optional |
| `vms` | `forward` | raw | `{"port": 8080}`, one of the ports the VM publishes (its `spec.ports`) |
| `tasks` | `attach` | terminal | none |

A container is reached through its Docker VM: `exec` runs `docker exec`,
`docker cp` or a relay (`nc`) there, so a container needs nothing of its own.

## Permissions

Each action of a VM's is opened under a permission of its own, one on one's
own VMs and one on anybody's. The node holding the VM decides from the
token's `permissions` claim alone:

| Action | On one's own VM | On somebody else's |
|--------|-----------------|--------------------|
| `attach` | `self.workload.vms.attach` or `workload.vms.attach` | `workload.vms.attach` |
| `exec` | `self.workload.vms.exec` or `workload.vms.exec` | `workload.vms.exec` |
| `forward` | `self.workload.vms.forward` or `workload.vms.forward` | `workload.vms.forward` |

- On one's own VM, either permission opens it: the self one or the
  anybody's.
- On somebody else's, only the anybody's one does: whoever holds
  `workload.vms.exec` may run a command in anybody's VM.
- A stream refused is answered `403` to the VM's owner, and `404` to anybody
  else, as for a VM that is not there.

Whatever runs in a VM to reach something is `exec`'s: a VM's files (`tar`),
a container's command, terminal and files (`docker exec`, `docker attach`,
`docker cp`), and a relay (`nc`) to a port a Docker VM does not publish or
to a container's. A port a VM publishes is `forward`'s.

The CLI reaches one's own VMs and containers, and anybody's with `-A`
(`--all-users`), which finds them through the administrator's routes. A
stream refused with `403` is said as the permission it needs, the self one
without `-A` and the anybody's with it (`access.StreamPermission`); a
published port refused with `403` is not relayed instead, since the
permission to forward it is what is missing.

## The terminal protocol

Binary messages are the terminal's bytes: the client's are its input, the
server's its output. A text message from the client is a control message:
`{"type": "resize", "rows": 40, "cols": 120}`. The server closes the
websocket when the shell ends.

## The channel protocol

Every message is binary. Its first byte is its channel and the rest is its
data:

| Channel | Direction | Data |
|---------|-----------|------|
| `0` stdin | client → server | input for the command |
| `1` stdout | server → client | the command's output |
| `2` stderr | server → client | the command's errors; nothing under a terminal, where all of it is on stdout |
| `3` exit | server → client | `{"exit_code": 0}` once the command has ended and all of its output was sent, or `{"exit_code": -1, "error": "..."}` when it could not be waited for; the server then closes the websocket with 1000 |
| `4` resize | client → server | `{"rows": 40, "cols": 120}` |
| `5` close | client → server | nothing: the end of the input, which closes the command's stdin |

Text messages are ignored. A client that closes the websocket ends the
command.

## The raw protocol

Binary messages, both ways, are the bytes of the TCP connection, in order.
Either side closing the websocket closes the connection. The server closes
with 1000 when the other end of the connection closed it, and with 1011 and
the reason when it could not be dialled.
