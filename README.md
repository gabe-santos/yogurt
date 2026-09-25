# Yogurt

A modern, minimal feed reader.

Run it as a [Server](#run-a-server) on a machine you control. A Desktop App is coming later.

## Run a Server

You need a computer or VPS with [Docker](https://docs.docker.com/get-docker/) installed. Compose comes with it.

**1.** Make a folder for Yogurt and save this in it as `compose.yaml`:

```yaml
services:
  yogurt:
    image: ghcr.io/gabe-santos/yogurt:latest
    restart: unless-stopped
    stop_grace_period: 15s
    environment:
      YOGURT_PASSWORD: ${YOGURT_PASSWORD:?set YOGURT_PASSWORD in .env}
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - data:/data

volumes:
  data:
```

**2.** In the same folder, create a `.env` file holding the password you will sign in with:

```sh
echo 'YOGURT_PASSWORD=choose-a-long-password' > .env
```

**3.** Start Yogurt:

```sh
docker compose up -d
```

On that machine, open <http://localhost:8080> and sign in. Yogurt starts again by itself after a reboot, and your Feeds and Entries are kept in a Docker volume that survives restarts and updates.

### Use it from other devices (HTTPS)

Yogurt only accepts connections from the machine it runs on. To reach it from your phone or laptop, put a reverse proxy with HTTPS in front of it. With [Caddy](https://caddyserver.com) installed on the same machine and your domain pointing at it, this is the whole `Caddyfile`, and Caddy gets the certificate for you:

```caddyfile
yogurt.example.com {
	reverse_proxy localhost:8080
}
```

Traefik, nginx or any other proxy installed on the same machine works too: point it at `localhost:8080`. Have it send the `X-Forwarded-Proto` header as well, so Yogurt knows the connection is HTTPS and your browser only ever sends your login cookie over HTTPS. Caddy and Traefik send it by themselves; in nginx, add `proxy_set_header X-Forwarded-Proto $scheme;`.

### Update

```sh
docker compose pull
docker compose up -d
```

### Back up and restore

Stop Yogurt before copying, because its latest changes are only fully written to the database when it shuts down. Each backup lands in a new folder:

```sh
docker compose stop
docker compose cp yogurt:/data "./backup-$(date +%Y%m%d-%H%M%S)"
docker compose start
```

To restore, use your backup folder's name in place of `backup-20260924-120000`:

```sh
docker compose stop
docker run --rm --user 65532 --volumes-from "$(docker compose ps -aq yogurt)" \
  -v "$PWD/backup-20260924-120000:/backup" busybox sh -c 'rm -f /data/* && cp /backup/* /data/'
docker compose start
```

## Contributing

Issues are welcome; pull requests aren't accepted yet. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
