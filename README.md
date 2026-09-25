# Yogurt

## Run a Server

A Server needs Docker with Compose. From a clone of this repository, put the password in a `.env` file next to `compose.yaml`:

```sh
echo 'YOGURT_PASSWORD=choose-a-long-password' > .env
docker compose up -d
```

The Web App is now at <http://localhost:8080>. Everything Yogurt stores lives in the `data` volume, which survives `docker compose down` and rebuilds. To update, pull and rebuild: `git pull && docker compose up -d --build`.

### HTTPS

The compose file runs only Yogurt, and publishes it on `127.0.0.1:8080` so the plain-HTTP port is not reachable from other machines. HTTPS comes from your own reverse proxy on the same machine. With [Caddy](https://caddyserver.com), which fetches the certificate itself, the whole `Caddyfile` is:

```caddyfile
yogurt.example.com {
	reverse_proxy localhost:8080
}
```

Traefik or any other proxy on the machine works the same way: point it at `localhost:8080`. If the proxy runs in Docker itself, `localhost` there is the proxy's own container. Instead, put Yogurt on the proxy's network with a `compose.override.yaml` beside `compose.yaml`, which Compose merges in, and point the proxy at `yogurt:8080`:

```yaml
services:
  yogurt:
    networks: [proxy]
networks:
  proxy: # your proxy's Docker network, by its exact name
    external: true
```

### Backups

Yogurt's database is `yogurt.db` in the data volume. Recent writes sit beside it in `yogurt.db-wal` until Yogurt shuts down, so a copy taken while it runs can miss them. Stop it for the copy, which lands in a new timestamped folder each time:

```sh
docker compose stop
docker compose cp yogurt:/data "./yogurt-backup-$(date +%Y%m%d-%H%M%S)"
docker compose start
```

To restore, replace the data volume's contents with a backup folder. Yogurt runs as user `65532`, which must own the files:

```sh
backup=yogurt-backup-20260924-120000  # the folder to restore
docker compose stop
docker run --rm --user 65532 --volumes-from "$(docker compose ps -aq yogurt)" -v "$PWD/$backup:/backup" \
  busybox sh -c 'rm -f /data/* && cp /backup/* /data/'
docker compose start
```
