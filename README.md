# WireGuard Docker Tunnel to HTTP Proxy Server

Converts WireGuard connection to HTTP proxy server in Docker. This allows you to have multiple WireGuard to HTTP proxies in different containers and expose them to different host ports.

Supports latest Docker for Windows, Linux, and MacOS.

## What it does?

1. It reads in a WireGuard configuration file (`.conf`) from a mounted file, specified through `WIREGUARD_CONFIG` environment variable.
2. If such configuration file is not provided, it will try to generate one in the following steps:
    - If all the following environment variables are set, it will use them to generate a configuration file:
        - `WIREGUARD_INTERFACE_PRIVATE_KEY`
        - `WIREGUARD_INTERFACE_DNS` defaults to `1.1.1.1`
        - `WIREGUARD_INTERFACE_ADDRESS`
        - `WIREGUARD_PEER_PUBLIC_KEY`
        - `WIREGUARD_PEER_ALLOWED_IPS` defaults to `0.0.0.0/0`
        - `WIREGUARD_PEER_ENDPOINT`
    - Otherwise, it will generate a free Cloudflare Warp account and use that as a configuration.
3. It starts the WireGuard client program to establish the VPN connection.
4. It optionally runs the executable defined by `WIREGUARD_UP` when the VPN connection is stable.
5. It starts the **HTTP Proxy** server and listens on container-scoped port **8080** by default. Proxy authentication can be enabled with `PROXY_USER` and `PROXY_PASS` environment variables. `PROXY_PORT` can be used to change the default port.
6. It optionally runs the executable defined by `PROXY_UP` when the HTTP proxy server is ready.
7. It optionally runs the user specified CMD line from `docker run` positional arguments ([see Docker doc](https://docs.docker.com/engine/reference/run/#cmd-default-command-or-options)). The program will use the VPN connection inside the container.
8. If user has provided CMD line, and `DAEMON_MODE` environment variable is not set to `true`, then after running the CMD line, it will shutdown the OpenVPN client and terminate the container.


### Deployment on Zeabur / PaaS

If you are deploying on Zeabur or other PaaS that use ingress routing based on the Host header, standard HTTP CONNECT requests might fail with 404. You need to spoof the Host header in the proxy request.

**Usage with curl:**

```bash
# Replace 'your-app.zeabur.app' with your actual domain
curl -v -x https://your-app.zeabur.app \
     --proxy-header "Host: your-app.zeabur.app" \
     https://ifconfig.me
```

**Why?**
Standard CONNECT requests set the `Host` header to the *destination* (e.g., `ifconfig.me`). PaaS platforms route traffic based on the Host header, so they don't know where to send the CONNECT request. By manually setting the Host header to your app's domain, the PaaS routes it correctly. The updated server code (v2) knows to ignore this spoofed Host header and use the Request-URI for dialing.

## Example with Warp


```bash

# Unix
SET NAME="myproxy"
PORT="8080"
USER="myuser"
PASS="mypass"
docker run --name "${NAME}" -dit --rm \
    --device=/dev/net/tun --cap-add=NET_ADMIN --privileged \
    -p "${PORT}":8080 \
    -e PROXY_USER="${USER}" \
    -e PROXY_PASS="${PASS}" \
    curve25519xsalsa20poly1305/wireguard-http-proxy \
    curl ifconfig.me

# Windows
SET NAME="myproxy"
SET PORT="8080"
SET USER="myuser"
SET PASS="mypass"
docker run --name "%NAME%" -dit --rm ^
    --device=/dev/net/tun --cap-add=NET_ADMIN --privileged ^
    -p "%PORT%":8080 ^
    -e PROXY_USER="%USER%" ^
    -e PROXY_PASS="%PASS%" ^
    curve25519xsalsa20poly1305/wireguard-http-proxy ^
    curl ifconfig.me
```

Then on your host machine test it with curl:

```bash
# Unix & Windows
curl ifconfig.me -x http://myuser:mypass@127.0.0.1:8080
```

To stop the daemon, run this:

```bash
# Unix
NAME="myproxy"
docker stop "${NAME}"

# Windows
SET NAME="myproxy"
docker stop "%NAME%"
```

### Example with Config File

Prepare a WireGuard configuration at `./wg.conf`. NOTE: DO NOT use IPv6 related configs as they may not be supported in Docker.

```bash
# Unix
docker run -it --rm \
    --device=/dev/net/tun --cap-add=NET_ADMIN --privileged \
    -v "${PWD}":/vpn:ro -e WIREGUARD_CONFIG=/vpn/wg.conf \
    curve25519xsalsa20poly1305/wireguard-http-proxy \
    curl ifconfig.me

# Windows
docker run -it --rm ^
    --device=/dev/net/tun --cap-add=NET_ADMIN --privileged ^
    -v "%CD%":/vpn:ro -e WIREGUARD_CONFIG=/vpn/wg.conf ^
    curve25519xsalsa20poly1305/wireguard-http-proxy ^
    curl ifconfig.me
```

## Contributing

Please feel free to contribute to this project. But before you do so, just make
sure you understand the following:

1\. Make sure you have access to the official repository of this project where
the maintainer is actively pushing changes. So that all effective changes can go
into the official release pipeline.

2\. Make sure your editor has [EditorConfig](https://editorconfig.org/) plugin
installed and enabled. It's used to unify code formatting style.

3\. Use [Conventional Commits 1.0.0-beta.2](https://conventionalcommits.org/) to
format Git commit messages.

4\. Use [Gitflow](https://www.atlassian.com/git/tutorials/comparing-workflows/gitflow-workflow)
as Git workflow guideline.

5\. Use [Semantic Versioning 2.0.0](https://semver.org/) to tag release
versions.

## License

Copyright © 2019 curve25519xsalsa20poly1305 &lt;<curve25519xsalsa20poly1305@gmail.com>&gt;

This work is free. You can redistribute it and/or modify it under the
terms of the Do What The Fuck You Want To Public License, Version 2,
as published by Sam Hocevar. See the COPYING file for more details.
