# GoOmni

One self-hosted Go gateway for independent **Bale, Eitaa and Rubika** accounts.
Use a shared REST API, durable send queues, scheduling and signed webhooks from
your backend. The English/Persian account and webhook panel is embedded in the
binary. Sponsored by [MuChat](https://mu.chat); independent MIT open-source software.

Version 2.4.1 corrects Eitaa/Rubika recovery boundaries and sender-name enrichment,
isolates unavailable Rubika peers and fixes missing-size avatar downloads. It
preserves storage schema 9, the shared SQLite database and accepted work.

- [Source and quickstart](https://github.com/mimalef70/goomni)
- [Release 2.4.1 and checksums](https://github.com/mimalef70/goomni/releases/tag/v2.4.1)
- [API reference](https://mimalef70.github.io/goomni/)
- [Consumer integration](https://github.com/mimalef70/goomni/blob/v2.4.1/docs/consumer-integration.md)
- [Upgrade from GoBale](https://github.com/mimalef70/goomni/blob/v2.4.1/docs/upgrade-goomni.md)

## Images

```sh
docker pull mimalef70/goomni:v2.4.1
```

The versioned image supports **Linux amd64 and arm64** and is also published as
`ghcr.io/mimalef70/goomni:v2.4.1`. Both registries contain the same immutable build.
Use an explicit version tag or the digest from the release notes. The runtime is
one non-root Go process with SQLite; Python, Node, a browser, Redis and PostgreSQL
are not runtime requirements. License notices are included in the image.

## Build from source with Docker Compose

The source Dockerfile uses Docker Hub images for all build and runtime stages;
it does not require access to `gcr.io`. The final runtime is Debian slim with
only CA certificates added. The Go process still runs as UID/GID 65532, with a
read-only root filesystem and dropped Linux capabilities.

The Compose file deliberately joins the pre-existing `internal_net` network so
the Sepehr panel can reach GoOmni at `http://eita-rubika-bale:1598`. Create the
network once if it does not already exist:

```sh
docker network inspect internal_net >/dev/null 2>&1 || docker network create internal_net
```

For an existing installation, keep its current `.env`, `master.key`, database,
media and named volume. Do not run `init` again and do not run `docker compose down -v`.

Validate the Compose file and build/start the service:

```sh
docker compose config --quiet
docker compose build --pull goomni
docker compose up -d --no-build goomni
docker compose ps
docker compose logs --tail=100 goomni
```

The service also has the network alias `eita-rubika-bale` for compatibility with
existing Sepehr panel configurations. The host port defaults to
`127.0.0.1:1598`; container-to-container traffic should use the network alias
and port 1598, not the host-published port.

## New installation

Download [docker-compose.yml](https://raw.githubusercontent.com/mimalef70/goomni/v2.4.1/docker-compose.yml)
into a private installation directory, then run:

```sh
export APP_IMAGE='docker.io/mimalef70/goomni:v2.4.1'
docker pull "$APP_IMAGE"
docker run --rm --user "$(id -u):$(id -g)" \
  --volume "$PWD:/config" --workdir /config \
  "$APP_IMAGE" init --bale-web-client
export APP_MASTER_KEY="$(cat master.key)"
docker compose up -d --no-build
docker compose logs --tail=100 goomni
```

Open `http://127.0.0.1:1598/ui/` and use the generated `APP_BASIC_AUTH`. Connect an
account using its own phone/code/password flow. The credential is gateway-wide
administration: keep it in your backend, never in an end user's browser.

Bale is enabled by default. Add `EITAA_ENABLED=true` and/or
`RUBIKA_ENABLED=true` to the local `.env` to opt in, then recreate the container.
Read `GET /app/providers` and the selected account's capability response before
offering a feature. Public protocol changes can affect any unofficial adapter.

## Storage and upgrades

**Existing GoOmni or GoBale installation? Follow the upgrade guide before starting.** Stop
the old service and make a full manual snapshot. Keep its original database,
media, encryption key and actual Docker volume. Set `APP_DATA_VOLUME` to that
volume and `APP_DATABASE` to its original container path, often
`/app/storages/gobale.db`. Do not rerun `init` or remove the old volume.

GoOmni 2.4.1 upgrades schema 8 to 9 transactionally on startup, backfilling local
conversation summaries. Allow time and temporary disk space for a large event
journal. Rolling back to a binary requiring an older storage schema needs its
matching pre-upgrade snapshot.

New installations use `goomni-data` and `/app/storages/goomni.db`. Run exactly one
process per database/media root. Sessions and stored secrets are encrypted;
ordinary event bodies and media require filesystem/volume protection. History
and registered media have no automatic expiry. Back up the full storage and
matching master key, and monitor disk usage.

## Verified scope

Controlled two-account tests cover native login, restart, selected messaging,
media, group/channel operations and signed webhooks. A one-hour Eitaa/Rubika
process outage recovered all controlled test messages. This does not establish
months-old archives, multi-day recovery, every operation or 300 live accounts.
Eitaa/Rubika remain opt-in; Eitaa poll/typing and some media presentation variants
retain documented limits. Unknown sends are not automatically retried.

Read the [dated acceptance record](https://github.com/mimalef70/goomni/blob/v2.4.1/docs/providers/acceptance.md)
for exact builds, failures and remaining gates. Report bugs in
[GitHub issues](https://github.com/mimalef70/goomni/issues) without phone numbers,
OTPs, tokens, sessions, databases or private messages.
