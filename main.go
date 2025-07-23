package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "syscall"
)

func main() {
    switch os.Args[1] {
    case "run":
        run()
    case "child":
        child()
    default:
        panic("Unknown command")
    }
}

func run() {
    cmd := exec.Command("/proc/self/exe", append([]string{"child"}, os.Args[2:]...)...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUTS |
            syscall.CLONE_NEWPID |
            syscall.CLONE_NEWNS |
            syscall.CLONE_NEWNET |
            syscall.CLONE_NEWIPC,
    }

    must(cmd.Start(), "starting child container process")
    fmt.Printf("Container PID: %d\n", cmd.Process.Pid)

    setupNetwork(cmd.Process.Pid)
    must(cmd.Wait(), "waiting for container")
}

func setupNetwork(pid int) {
    fmt.Println("Setting up veth pair...")

    fmt.Println("-> Creating veth pair")
    must(exec.Command("ip", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1").Run(), "create veth pair")

    fmt.Println("-> Moving veth1 to container netns")
    must(exec.Command("ip", "link", "set", "veth1", "netns", fmt.Sprint(pid)).Run(), "move veth1")

    fmt.Println("-> Assigning IP to veth0")
    must(exec.Command("ip", "addr", "add", "10.0.0.1/24", "dev", "veth0").Run(), "assign IP to veth0")

    fmt.Println("-> Bringing up veth0")
    must(exec.Command("ip", "link", "set", "veth0", "up").Run(), "bring up veth0")

    fmt.Println("-> Assigning IP to veth1 (container side)")
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n", "ip", "addr", "add", "10.0.0.2/24", "dev", "veth1").Run(), "assign IP to veth1")

    fmt.Println("-> Bringing up veth1")
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n", "ip", "link", "set", "veth1", "up").Run(), "bring up veth1")

    fmt.Println("-> Bringing up loopback")
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n", "ip", "link", "set", "lo", "up").Run(), "bring up lo")

    fmt.Println("-> Adding default route")
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n", "ip", "route", "add", "default", "via", "10.0.0.1").Run(), "add default route")

    fmt.Println("Network setup complete")
}

func child() {
    fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())

    // ✅ Set hostname inside UTS namespace
    must(syscall.Sethostname([]byte("container")), "set hostname")

    rootfs := "/home/chris/containers-from-scratch/alpine-rootfs"

    must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""), "remount / as private")

    must(os.MkdirAll(filepath.Join(rootfs, "proc"), 0755), "create /proc")
    must(syscall.Mount("proc", filepath.Join(rootfs, "proc"), "proc", 0, ""), "mount proc")

    must(os.MkdirAll(filepath.Join(rootfs, "etc"), 0755), "mkdir /etc")
    must(syscall.Mount("/etc/resolv.conf", filepath.Join(rootfs, "etc/resolv.conf"), "", syscall.MS_BIND, ""), "bind mount resolv.conf")

    must(os.MkdirAll(filepath.Join(rootfs, "old_root"), 0755), "create old_root")
    must(syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""), "bind mount new rootfs")
    must(syscall.Chdir(rootfs), "chdir into new root")
    must(syscall.PivotRoot(".", "old_root"), "pivot_root")
    must(syscall.Chdir("/"), "chdir / after pivot")

    must(syscall.Unmount("/old_root", syscall.MNT_DETACH), "unmount old_root")
    must(os.Remove("/old_root"), "remove old_root dir")

    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Credential: &syscall.Credential{Uid: 0, Gid: 0},
    }

    must(cmd.Run(), "run command inside container")
}

func must(err error, msg string) {
    if err != nil {
        log.Fatalf("ERROR [%s]: %v", msg, err)
    }
}
