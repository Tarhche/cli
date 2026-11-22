package workload

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/Tarhche/cli/internal/workload"
)

// publishedOf is the port docker's --publish says: a container's port, and
// the port and the address of its VM it is published on,
// [ip:][host port:]container port[/protocol]. A host port left out lets
// docker pick one.
func publishedOf(spec string) (workload.PortBinding, error) {
	invalid := fmt.Errorf("%q is not a port to publish: give [ip:][host port:]container port[/protocol]", spec)

	rest, protocol, _ := strings.Cut(spec, "/")

	var hostIP string

	if strings.HasPrefix(rest, "[") {
		closing := strings.Index(rest, "]:")
		if closing < 0 {
			return workload.PortBinding{}, invalid
		}

		hostIP, rest = rest[1:closing], rest[closing+2:]
	}

	parts := strings.Split(rest, ":")

	var hostPort, containerPort string

	switch len(parts) {
	case 1:
		containerPort = parts[0]
	case 2:
		hostPort, containerPort = parts[0], parts[1]
	case 3:
		if len(hostIP) > 0 {
			return workload.PortBinding{}, invalid
		}

		hostIP, hostPort, containerPort = parts[0], parts[1], parts[2]
	default:
		return workload.PortBinding{}, invalid
	}

	binding := workload.PortBinding{HostIP: hostIP, Protocol: protocol}

	port, err := strconv.ParseUint(containerPort, 10, 16)
	if err != nil {
		return workload.PortBinding{}, invalid
	}

	binding.ContainerPort = uint(port)

	if len(hostPort) > 0 {
		port, err := strconv.ParseUint(hostPort, 10, 16)
		if err != nil {
			return workload.PortBinding{}, invalid
		}

		binding.HostPort = uint(port)
	}

	return binding, nil
}

// volumeOf is what docker's --volume mounts: a volume of the VM by its name,
// or a path of the VM, [source:]target[:ro|rw].
func volumeOf(spec string) (workload.Mount, error) {
	parts := strings.Split(spec, ":")

	var source, target, options string

	switch {
	case len(parts) == 1:
		target = parts[0]
	case len(parts) == 2 && (parts[1] == "ro" || parts[1] == "rw"):
		target, options = parts[0], parts[1]
	case len(parts) == 2:
		source, target = parts[0], parts[1]
	case len(parts) == 3:
		source, target, options = parts[0], parts[1], parts[2]
	default:
		return workload.Mount{}, fmt.Errorf("%q is not a volume to mount: give [source:]target[:ro|rw]", spec)
	}

	mount := workload.Mount{Type: workload.MountVolume, Source: source, Target: target}
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
		mount.Type = workload.MountBind
	}

	for option := range strings.SplitSeq(options, ",") {
		switch option {
		case "", "rw":
		case "ro", "readonly":
			mount.ReadOnly = true
		default:
			return workload.Mount{}, fmt.Errorf("%q is not a volume option: give ro or rw", option)
		}
	}

	return mount, nil
}

// mountOf is what docker's --mount mounts:
// type=volume|bind|tmpfs,source=...,target=...,readonly.
func mountOf(spec string) (workload.Mount, error) {
	mount := workload.Mount{Type: workload.MountVolume}

	for field := range strings.SplitSeq(spec, ",") {
		key, value, valued := strings.Cut(field, "=")

		switch strings.ToLower(key) {
		case "type":
			mount.Type = value
		case "source", "src":
			mount.Source = value
		case "target", "destination", "dst":
			mount.Target = value
		case "readonly", "ro":
			if !valued {
				mount.ReadOnly = true

				continue
			}

			readOnly, err := strconv.ParseBool(value)
			if err != nil {
				return workload.Mount{}, fmt.Errorf("%q is not a mount: readonly is true or false", spec)
			}

			mount.ReadOnly = readOnly
		default:
			return workload.Mount{}, fmt.Errorf("%q is not a mount: %q is none of type, source, target and readonly", spec, key)
		}
	}

	return mount, nil
}

// tmpfsOf is what docker's --tmpfs mounts: a tmpfs at a path, its options
// after a colon, which a VM's dockerd is not given.
func tmpfsOf(spec string) workload.Mount {
	target, _, _ := strings.Cut(spec, ":")

	return workload.Mount{Type: workload.MountTmpfs, Target: target}
}

// environment is what docker's --env-file and --env say a container runs
// with, in that order: KEY=value, or KEY alone for its value here, which is
// left out when it has none.
func environment(files [][]byte, given []string, lookup func(string) (string, bool)) []string {
	var env []string

	add := func(variable string) {
		if strings.Contains(variable, "=") {
			env = append(env, variable)

			return
		}

		if value, found := lookup(variable); found {
			env = append(env, variable+"="+value)
		}
	}

	for _, content := range files {
		lines := bufio.NewScanner(bytes.NewReader(content))
		for lines.Scan() {
			line := strings.TrimSpace(lines.Text())
			if len(line) > 0 && !strings.HasPrefix(line, "#") {
				add(line)
			}
		}
	}

	for _, variable := range given {
		add(variable)
	}

	return env
}
