# docker-credential-1password

Use 1Password as read-only credential store for the Docker CLI.
This avoids storing registry credentials on disk.

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

1. Download the latest release from 
     [GitHub Releases](https://github.com/reitzig/docker-credential-1password/releases).
   Make sure the binary is executable and in your `PATH`.

   - As an alternative, you can also clone the repository and run

     ```bash
     go install github.com/reitzig/docker-credential-1password@<version>
     ```
2. Now configure `docker` to use the helper for all registries:

   ```json5
   // .docker/config.json
   {
     "credsStore": "1password",
     "auths": {}
   }
   ```

   > [!TIP]
   > If you need to mix and match 1password with other credential stores, 
   > please refer to the 
   >   [official Docker documentation](https://docs.docker.com/reference/cli/docker/login/#configure-credential-helpers)
   > on how to configure credential _helpers_ instead of a _store_.

3. Finally, configure the 1Password desktop app to integrate with applications that use the SDK by
checking 'Settings > Developer > Developer Integrations > Integrate with 1Password SDKs'.


## Usage

Next to your `.docker/config.json`, create a file `credential-1password.json` and
add references to all necessary secrets.
For example, you would create this for DockerHub:

```json5
{
  "account": "<account name or uuid>", // (1)
  "secretRefs": {
    "https://index.docker.io/v1": {
      "username": {
        "vault": "<vault id>", // (2)
        "item": "<item id or name>",
        "field": "<field id or name>" // (3)
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
> 2. You _have_ to use the vault ID here; the name doesn't work.
>    You can determine it by
>    - right-clicking on the vault name in the sidebar of the desktop app, or
>    - by running
>      ```bash
>      op vault list --format json
>      ```
> 3. Field names may contain section names, i.e. `<section name>/<field name>`.
> 
> > [!TIP]
> > If you use the 'Copy Secret Reference' feature on any item in the desktop app,
> > you can read off everything you need except for the vault ID:
> > ```
> > op://<vault name>/<item name>/<field name>"
> > ```

Run
```bash
docker-credential-1password list
```
to check the configuration.

Now simply run `docker pull` or any other command that requires authentication;
it will automatically use the 1Password helper to retrieve credentials.

> [!IMPORTANT]
> Do not run `docker login`!
>
> After failing to write using the helper, it will fall back to asking for credentials and storing them on disk.


## Alternatives

- [xebia/docker-credential-1password](https://github.com/xebia/docker-credential-1password)
  -- a full credential store implementation using the `op` CLI.
- Develop a [1Password Shell Plugin](https://www.1password.dev/cli/shell-plugins) wrapping `docker`.

## Notes

No maintainer has any affiliation with AgileBits Inc.
