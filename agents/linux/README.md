# Gaia Linux Agent

The Gaia Linux agent is an opt-in nervous-system endpoint.

Current version emits aggregated host-health signals:

- 1, 5, and 15 minute CPU load averages
- System uptime
- CPU count
- Linux kernel release and eBPF-readiness metadata

It does not collect process names, command lines, file paths, user data, or network payloads.

> eBPF collection will be added later as a separate privileged capability.

## Build from Windows

From the repository root:

```powershell
.\scripts\build-linux-agent.ps1