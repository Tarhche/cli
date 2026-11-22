package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/Tarhche/cli/internal/portforward"
)

// addressUsage is the usage of port-forward's --address, kubectl's.
const addressUsage = "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, it binds to both 127.0.0.1 and ::1"

// portSpec is a port forwarded: the local one, 0 for any that is free, and
// the remote one.
type portSpec struct {
	local  uint16
	remote uint
}

// parsePorts reads [LOCAL_PORT:]REMOTE_PORT arguments.
func parsePorts(args []string) ([]portSpec, error) {
	specs := make([]portSpec, 0, len(args))

	for _, arg := range args {
		local, remote, pair := strings.Cut(arg, ":")
		if !pair {
			local, remote = arg, arg
		}

		remotePort, err := strconv.ParseUint(remote, 10, 16)
		if err != nil || remotePort == 0 {
			return nil, fmt.Errorf("invalid port %q: the remote port has to be a number from 1 to 65535", arg)
		}

		var localPort uint64

		if len(local) > 0 {
			if localPort, err = strconv.ParseUint(local, 10, 16); err != nil {
				return nil, fmt.Errorf("invalid port %q: the local port has to be a number from 0 to 65535", arg)
			}
		}

		specs = append(specs, portSpec{local: uint16(localPort), remote: uint(remotePort)})
	}

	return specs, nil
}

// listenAddress is an address forwards listen on; one that may not be
// there, ::1 where IPv6 is not, is left out when it cannot be listened on.
type listenAddress struct {
	ip       string
	optional bool
}

// parseAddresses reads --address: IP addresses, and localhost, which is
// 127.0.0.1 and ::1, separated by commas.
func parseAddresses(value string) ([]listenAddress, error) {
	var addresses []listenAddress

	for address := range strings.SplitSeq(value, ",") {
		address = strings.TrimSpace(address)

		switch {
		case address == "localhost":
			addresses = append(addresses, listenAddress{ip: "127.0.0.1"}, listenAddress{ip: "::1", optional: true})
		case net.ParseIP(address) != nil:
			addresses = append(addresses, listenAddress{ip: address})
		default:
			return nil, fmt.Errorf("%q is not an IP address or localhost", address)
		}
	}

	return addresses, nil
}

// forward forwards every port on every address until ctx is done, or a
// forward cannot accept connections any more, each connection's other side
// opened by what dial gives for its remote port. Its forwards, and every
// connection, are told as kubectl tells them.
func (r *remote) forward(ctx context.Context, addresses []listenAddress, ports []portSpec, dial func(remote uint) portforward.Dial) error {
	var lock sync.Mutex

	say := func(w io.Writer, format string, args ...any) {
		lock.Lock()
		defer lock.Unlock()

		_, _ = fmt.Fprintf(w, format, args...)
	}

	told := portforward.WithEvents(func(event portforward.Event) {
		switch event.Kind {
		case portforward.Accepted:
			say(r.cli.Out, "Handling connection for %d\n", portOf(event.Forward.Addr()))
		case portforward.Failed:
			say(r.cli.Err, "error forwarding %s -> %s: %s\n", event.Forward.Addr(), event.Forward.Remote(), event.Err)
		}
	})

	forwarder := portforward.New(append([]portforward.Option{told}, r.forwarding...)...)

	defer func() { _ = forwarder.Close() }()

	var forwards []*portforward.Forward

	for _, port := range ports {
		local := port.local

		for _, address := range addresses {
			spec := portforward.Spec{Address: address.ip, Port: local, Remote: strconv.FormatUint(uint64(port.remote), 10), Dial: dial(port.remote)}

			started, err := forwarder.Start(spec)
			if err != nil {
				if address.optional {
					continue
				}

				return err
			}

			local = portOf(started.Addr())
			forwards = append(forwards, started)

			say(r.cli.Out, "Forwarding from %s -> %d\n", started.Addr(), port.remote)
		}
	}

	stopped := make(chan *portforward.Forward, len(forwards))
	for _, started := range forwards {
		go func() {
			<-started.Done()
			stopped <- started
		}()
	}

	select {
	case <-ctx.Done():
		return nil
	case started := <-stopped:
		return fmt.Errorf("forwarding %s stopped: %w", started.Addr(), started.Err())
	}
}

// portOf is the port of a TCP address.
func portOf(address net.Addr) uint16 {
	if tcp, ok := address.(*net.TCPAddr); ok {
		return uint16(tcp.Port)
	}

	return 0
}
