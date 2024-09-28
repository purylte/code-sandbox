# Code Sandbox

A Web UI to build and run code safely inside gvisor docker container. Contains a ui web server, a container manager builder api server, and a container manager runner api server.

# Development
## Using Dev Container (VS Code)
1. Ensure [Docker](https://www.docker.com/) and [Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension is installed
2. Open this project in VS Code
```bash
git clone https://github.com/purylte/code-sandbox.git
code code-sandbox
```
3. Run "Dev Containers: Reopen in Container" in VS Code
4. Install required js library by installing and copying to a directory by running
```bash
make init-vendor
```
5. Run `./dev.sh` to start developing
## Manually
1. Clone the repository
```bash
git clone https://github.com/purylte/code-sandbox.git
cd code-sandbox
```
2. Install pnpm
3. Install required js library by installing and copying to a directory by running
```bash
make init-vendor
```
4. Run `./dev.sh` to start developing


# Contributing

Feel free to fork this project, submit issues, and create pull requests. Contributions are welcome!

# License

This project is licensed under the MIT License - see the LICENSE file for details.