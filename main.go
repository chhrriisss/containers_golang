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

    must(cmd.Run())
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
