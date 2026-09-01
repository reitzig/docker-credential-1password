[![Go](https://img.shields.io/github/go-mod/go-version/reitzig/docker-credential-1password)](https://go.dev/)
[![MIT License](https://img.shields.io/github/license/reitzig/docker-credential-1password)](https://github.com/reitzig/docker-credential-1password/blob/main/LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/reitzig/docker-credential-1password)](https://github.com/reitzig/docker-credential-1password/releases/latest)
[![Checks](https://github.com/reitzig/docker-credential-1password/actions/workflows/check.yaml/badge.svg)](https://github.com/reitzig/docker-credential-1password/actions/workflows/check.yaml)

# docker-credential-1password

Use 1Password as read-only credential store for the Docker CLI.
This avoids storing registry credentials on disk or duplicating them to a second store.

> [!NOTE]
> This program is _not_ a full credential helper in the sense of 
>   [docker/docker-credential-helpers](https://github.com/docker/docker-credential-helpers/)
> as it implements neither `store` nor `erase`, by design.

### Security

Due to the sensitive nature of the data handled by this program,
we encourage you to review the source code before using it.

The _intent_ of the program is to pass credentials from 1Password (Desktop application) to `docker` CLI,
without further processing or persistent state beyond the config file (see below).

As such, overall security depends on
- the [security model of 1Password SDK](https://www.1password.dev/sdks/desktop-app-integrations), and
- how `docker` [calls and consumes the helper](https://docs.docker.com/reference/cli/docker/login/#credential-helper-protocol).

Of course, binary `docker-credential-1password` can be called by any other process as well.
No attempt is made here to mitigate this risk:
if the user authorizes such access through the 1Password app, or 
the app does so on their behalf, the calling process will be handed secrets.

Furthermore, by configuring the 1Password desktop app to integrate with _this_ program,
any other application using any of the 1Passworkd SDKs can (attempt to) connect as well.

## Installation

1.  Download the latest release from 
      [GitHub Releases](https://github.com/reitzig/docker-credential-1password/releases).
    Make sure the binary is executable and in your `PATH`.

    - As an alternative, you can also clone the repository and run

      ```bash
      go install github.com/reitzig/docker-credential-1password@<version>
      ```
2.  Now configure `docker` to use the helper for those registries 
    for which you require authentication;
    for instance, you can start with DockerHub:

    ```jsonc
    // .docker/config.json
    {
      "auths": {},
      "credHelpers": {
        "https://index.docker.io/v1/": "1password"
      }
    }
    ```
3.  Finally, configure the 1Password desktop app to integrate with applications that use the SDK by
checking 'Settings > Developer > Developer Integrations > Integrate with 1Password SDKs'.

> [!TIP]
> If you do not need other credential stores _and_
> never access any registries without authentication,
> you can avoid listing all registries by configuring a credentials _store_ instead:
>
> ```jsonc
> // .docker/config.json
> {
>   "credsStore": "1password",
>   "auths": {}
> }
> ```

### podman

`podman` (and related commands) can make use Docker credential helpers (cf. 
  [containers/image:docs/containers-auth.json.5.md](https://github.com/containers/image/blob/main/docs/containers-auth.json.5.md),
via 
  [docs.podman.io > login](https://docs.podman.io/en/latest/markdown/podman-login.1.html)).
You may have to create a separate config file:

```jsonc
// .config/containers/auth.json
{
  "credHelpers": {
    "docker.io": "1password"
  }
}
```

Note how `podman` refers to DockerHub in a different way compared to `docker`;
this will have to be reflected in your `credential-1password.json` as well (see below).

> [!NOTE]
> No catch-all setting like `credsStore` seems to exist here.

## Usage

> [!IMPORTANT]
> Do not run `docker login`!
>
> After failing to write using the helper, it will fall back to asking for credentials and storing them on disk.

Next to your `.docker/config.json`, create a file `credential-1password.json` and
add references to all necessary secrets.
For example, you would create this for DockerHub:

```jsonc
{
  "account": "<account name or uuid>", // (1)
  "secretRefs": {
    "index.docker.io/v1": { // (2)
      "username": {
        "vault": "<vault id>", // (3)
        "item": "<item id or name>",
        "field": "<field id or name>" // (4)
      },
      "secret": {
        "vault": "<vault id>",
        "item": "<item id or name>",
        "field": "<field id or name>"
      }
    }
  }
}
```

Add additional registries to `secretRefs` in a similar fashion.

> [!NOTE]
> 1. You can use the account name as shown in the desktop app, or `account_uuid` as per 
>    ```bash
>    op account list --format json
>    ```
> 2. While the exact URL that `docker` asks for certainly works, there are drawbacks;
>    for instance, `podman` may send a different URL for the same image!
>    You can use substrings, e.g. `docker.io` for DockerHub;
>    the most specific match will be used.
> 3. You _have_ to use the vault ID here; the name doesn't work.
>    You can determine it by
>    - right-clicking on the vault name in the sidebar of the desktop app, or
>    - by running
>      ```bash
>      op vault list --format json
>      ```
> 4. Field names may contain section names, i.e. `<section name>/<field name>`.

> [!TIP]
> If you use the 'Copy Secret Reference' feature on any item in the desktop app,
> you can read off everything you need except for the vault ID:
> ```
> op://<vault name>/<item name>/<field name>"
> ```

Run
```bash
docker-credential-1password list
```
to check the configuration.

Now simply run `docker pull` or any other command that requires authentication;
it will automatically use the 1Password helper to retrieve credentials.

Confirm the version you are using by running:

```bash
docker-credential-1password version
```

### Debugging

In case of any issues, 
set environment variable `DOCKER_CREDENTIAL_1PASSWORD_DEBUG=true` and 
re-run to inspect.

## Alternatives

- [xebia/docker-credential-1password](https://github.com/xebia/docker-credential-1password)
  -- a full credential store implementation using the `op` CLI.
- Develop a [1Password Shell Plugin](https://www.1password.dev/cli/shell-plugins) wrapping `docker`.

## Notes

- This program does not collect any data beyond its immediate purpose.
  It does not store any data outside its single configuration file.
  It does not transmit any data except through the 1Password SDK.
  - At the same time, we cannot be held responsible for data processing, storage, and transmission 
    performed by 1Password SDK. Refer to their privacy policy.
- "AI" coding assistance was used as noted in the commit messages.
  The human always remained in the loop.
- No maintainer has any affiliation with AgileBits Inc.
  1Password is their trademark.
