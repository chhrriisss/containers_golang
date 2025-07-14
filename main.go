package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

func main() {
	switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("bad command")
	}
}

func run() {
	fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())

	cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	must(cmd.Run())
}

func child() {
    fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())
    must(syscall.Sethostname([]byte("container")))

    must(os.MkdirAll("/proc", 0755))
    _ = syscall.Unmount("/proc", 0)
    must(syscall.Mount("proc", "/proc", "proc", 0, ""))

    must(os.MkdirAll("/sys/fs/cgroup", 0755))
    _ = syscall.Unmount("/sys/fs/cgroup", 0)
    must(syscall.Mount("none", "/sys/fs/cgroup", "cgroup2", 0, ""))

    cg()

    must(syscall.Chroot("./rootfs"))
    must(os.Chdir("/"))

    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    must(cmd.Run())
    _ = syscall.Unmount("/proc", 0)
}

func cg() {
    cgroups := "/sys/fs/cgroup"
    containerCgroup := filepath.Join(cgroups, "liz")

    must(os.MkdirAll(containerCgroup, 0755))

    must(os.WriteFile(filepath.Join(containerCgroup, "memory.max"), []byte("100000000"), 0700))
    must(os.WriteFile(filepath.Join(containerCgroup, "pids.max"), []byte("20"), 0700))
    must(os.WriteFile(filepath.Join(containerCgroup, "cgroup.procs"), []byte(strconv.Itoa(os.Getpid())), 0700))
}


func must(err error) {
	if err != nil {
		panic(err)
	}
}
