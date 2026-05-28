# mdns2hosts

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/errpunk/mdns2hosts/main/coverage-badge.json)](https://github.com/errpunk/mdns2hosts/actions)

Sync mDNS `.local` names to the system `hosts` file on Windows and Linux — automatically.

mdns2hosts resolves mDNS names directly via multicast DNS (bypassing system DNS) and writes the resulting IPv4 addresses into `# mdns2hosts`-tagged entries in the system hosts file. Other entries in the hosts file are never touched.

## Installation

### Download prebuilt binary

Go to [Releases](https://github.com/errpunk/mdns2hosts/releases) and download the Windows or Linux binary for your host.

### Build from source

```powershell
git clone https://github.com/errpunk/mdns2hosts.git
cd mdns2hosts
go build -o mdns2hosts.exe .
```

On Linux:

```bash
git clone https://github.com/errpunk/mdns2hosts.git
cd mdns2hosts
go build -o mdns2hosts .
```

Requires Go 1.25+.

### Cross-compile

```bash
GOOS=windows GOARCH=amd64 go build -o mdns2hosts.exe .
GOOS=linux GOARCH=amd64 go build -o mdns2hosts .
```

## Usage

### Sync mDNS names to hosts

```powershell
# Single name
mdns2hosts sync supercow.local

# Multiple names
mdns2hosts sync supercow.local devbox.local printer.local

# Preview the complete updated hosts file without writing it
mdns2hosts sync supercow.local --dry-run
```

`--dry-run` resolves the requested names, applies the same managed-entry update
logic as a real sync, and prints the full resulting hosts file content to the
terminal without modifying the file.

### Watch for IP changes continuously

```powershell
mdns2hosts watch supercow.local --interval 30s
```

Runs until interrupted (Ctrl+C). Updates hosts whenever an IP changes.

### Remove managed entries

```powershell
mdns2hosts clean
```

Removes all entries tagged with `# mdns2hosts`.

### Run as a Windows service

```powershell
# Install (requires Administrator)
mdns2hosts install-service --name supercow.local --interval 30s

# Start the service
sc start mdns2hosts

# Stop and remove
mdns2hosts uninstall-service
```

### Run as a Linux service

```bash
# Install (requires root)
sudo ./mdns2hosts install-service --name supercow.local --interval 30s

# Start and check the service
sudo systemctl start mdns2hosts
systemctl status mdns2hosts

# Stop and remove
sudo systemctl stop mdns2hosts
sudo ./mdns2hosts uninstall-service
```

## How it works

1. **mDNS query**: Sends a multicast DNS A-record query directly to `224.0.0.251:5353`. No system DNS resolution involved.
2. **Hosts update**: Writes resolved IPs into tagged entries in the hosts file (`C:\Windows\System32\drivers\etc\hosts` on Windows, `/etc/hosts` on Linux).
3. **Managed entries**: Only replaces lines tagged with `# mdns2hosts` — everything else is preserved.

```text
192.168.1.88 supercow.local # mdns2hosts
```

Updates are atomic (write to temp file, rename over real file).

## Requirements

- Windows 10+ / Windows Server 2016+, or a Linux distribution with a supported service manager such as systemd
- Administrator/root privileges (for `install-service`, `uninstall-service`, and writing to the hosts file)

## License

MIT
