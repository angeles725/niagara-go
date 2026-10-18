// Package procgroup runs a child process so that stopping it stops
// everything it started.
//
// CommandContext returns a *Cmd, an *exec.Cmd whose Start, Wait, Run,
// Output and CombinedOutput add, on Unix:
//
//   - The child runs in its own process group (Setpgid; an existing
//     SysProcAttr is kept) and the context's cancel (a timeout or the
//     caller's cancel) kills the whole group with SIGKILL, so a wrapper
//     script that forks its real program instead of exec'ing it leaves
//     nothing running. Plain exec.CommandContext kills only the direct
//     child.
//   - While the child runs, SIGINT and SIGTERM to this process are relayed:
//     the child's group does not get the terminal's Ctrl-C, so the signal
//     cancels the run instead (which kills the group) and Wait returns an
//     *InterruptError. A signal this process was started with ignored stays
//     ignored and is not relayed.
//   - By default Wait then re-raises the signal on this process so it dies
//     of it, as it would have without the relay (the shell sees 130 or 143).
//     Wait returns the *InterruptError only if the process outlives the
//     re-raised signal.
//
// Re-raising and other signal handlers: signal.Notify delivers a signal to
// every channel registered for it, so a handler of the caller's own already
// got the original signal, and the re-raised one reaches it again; a
// "press Ctrl-C twice to quit" handler would fire on one press. Whether
// another handler is registered cannot be told from outside the runtime, so
// a caller that registers its own handler for SIGINT or SIGTERM sets
// Cmd.ReturnInterrupt: Wait then returns the *InterruptError at once
// without re-raising and the caller decides how to stop. A caller with no
// handler of its own leaves it false and dies of the signal.
//
// On Linux the re-raise is a signal to the calling thread, delivered before
// the system call returns: the process is gone at that point, or, when
// another handler caught the signal, Wait returns at once. Other Unix
// systems signal the process and wait at most 250 ms for it to die.
//
// On Windows (and any other non-Unix system) a *Cmd behaves as the
// exec.Cmd it wraps: the cancel kills the direct child, which is how the
// current callers' children run (keytool.exe runs its JVM in-process),
// nothing is relayed (the console's Ctrl-C reaches every child directly)
// and Wait never returns an *InterruptError.
//
// A *Cmd must be run (Run, Output, CombinedOutput, or Start then Wait):
// Wait releases the context and the relay that Start set up.
package procgroup
