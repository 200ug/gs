# gs

## Usage

```
usage:
	gs init <user@host:port:/path>  initialize config with remote server
	gs track                        add current directory to sync list
	gs untrack                      remove current directory from sync list
	gs push                         sync local to server
	gs pull                         sync server to local
	gs status                       show pending changes (dry-run)
	gs auto [options]               wait for server, then pull all

auto options:
	--interval <duration>           poll interval (default: 30s)
	--timeout <duration>            max wait time, 0 for infinite (default: 15m)
```

## Config

The initial blank config can be generated to `~/.config/gs/gs.toml` with `gs init <user@host:port:/path>`. Directories can be added (or removed) to locals with `gs track` and `gs untrack`. Notably `gs` doesn't touch SSH configs which means the SSH pubkey auth to remote must be configured separately in `~/.ssh/config`.

Example config tracking locals `notes` and `documents` (and syncing them to `/srv/sync/notes` and `/srv/sync/documents`, respectively):

```
server = "user@host"
port = "22"
remote_path = "/srv/sync"
excludes = [".git", "*.tmp", ".gs.state"]

[[locals]]
name = "notes"
path = "/home/user/notes"

[[locals]]
name = "documents"
path = "/home/user/documents"
```

The auto-pull command can be configured to run on startup with e.g. a user-level systemd/launchd service. It'll try to sync the remote with configured locals, and quit after either succeeding in this task or by timing out (with the exception of `--timeout 0`).

