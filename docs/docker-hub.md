# Docker Hub Publishing

The repository includes `.github/workflows/docker-publish.yml`.

It builds `apps/bridge/Dockerfile` for `linux/amd64` and `linux/arm64` and
pushes:

```text
vincenzoimp/proxmox-desk-display:latest
vincenzoimp/proxmox-desk-display:main-<sha>
vincenzoimp/proxmox-desk-display:<semver tag>
```

The Docker Hub repository uses the product name rather than a component name
because this container is the standard runtime users install. Internally it
runs the bridge service.

Configure these GitHub repository secrets:

```text
DOCKERHUB_USERNAME
DOCKERHUB_TOKEN
```

`DOCKERHUB_TOKEN` should be a Docker Hub access token, not the account password.
Without these secrets, the workflow still builds the image but skips pushing.

Publishing happens on:

- pushes to `main`;
- tags like `v0.1.0`;
- manual workflow dispatch.

Users update with:

```bash
docker compose -f examples/docker-compose.yaml pull
docker compose -f examples/docker-compose.yaml up -d
```

The `/data` volume keeps admin UI config and secrets across image updates.
