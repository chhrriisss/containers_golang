package main

import (
    "fmt"
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

    // Step 1: Start the container process
    must(cmd.Start())
    fmt.Printf("Container PID: %d\n", cmd.Process.Pid)

    // Step 2: Set up networking
    setupNetwork(cmd.Process.Pid)

    // Step 3: Wait for the container to exit
    must(cmd.Wait())
}

func setupNetwork(pid int) {
    fmt.Println("Setting up veth pair...")

    // Create veth pair
    must(exec.Command("ip", "link", "add", "veth0", "type", "veth", "peer", "name", "veth1").Run())

    // Move veth1 to the container's network namespace
    must(exec.Command("ip", "link", "set", "veth1", "netns", fmt.Sprint(pid)).Run())

    // Setup host end: veth0
    must(exec.Command("ip", "addr", "add", "10.0.0.1/24", "dev", "veth0").Run())
    must(exec.Command("ip", "link", "set", "veth0", "up").Run())

    // Setup container end: veth1 (inside netns)
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n",
        "ip", "addr", "add", "10.0.0.2/24", "dev", "veth1").Run())
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n",
        "ip", "link", "set", "veth1", "up").Run())
    must(exec.Command("nsenter", "-t", fmt.Sprint(pid), "-n",
        "ip", "link", "set", "lo", "up").Run())

    fmt.Println(" Network setup complete")
}


func child() {
    fmt.Printf("Running %v as PID %d\n", os.Args[2:], os.Getpid())

    rootfs := "/home/chris/containers-from-scratch/rootfs"

    must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""))

    must(os.MkdirAll(filepath.Join(rootfs, "proc"), 0755))
    must(os.MkdirAll(filepath.Join(rootfs, "old_root"), 0755))

    // Make sure rootfs is a mount point
    must(syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""))

    must(syscall.Mount("proc", filepath.Join(rootfs, "proc"), "proc", 0, ""))

    must(syscall.Chdir(rootfs))
    must(syscall.PivotRoot(".", "old_root"))
    must(syscall.Chdir("/"))

    must(syscall.Unmount("/old_root", syscall.MNT_DETACH))
    must(os.Remove("/old_root"))

    cmd := exec.Command(os.Args[2], os.Args[3:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    must(cmd.Run())
}

func must(err error) {
    if err != nil {
        panic(err)
    }
}
