# HTTP File Server

Very very simple and lightweight HTTP file server for sharing files over a network.

## Overview

This HTTP File Server provides an easy way to share files and directories over HTTP. It creates a temporary web server that allows downloading/uploading/deleting files from the specified directory.

## Features

- Simple command-line interface
- Directory listing
- File downloads & uploads
- Cross-platform compatibility
- Helper scripts to build/run docker container

## Usage

### Running Locally

```bash
# Basic usage - serves current directory on port 8080
http-file-server

# Specify a custom port
http-file-server --listen-port 9000

# Serve a specific directory
http-file-server --dir-to-serve /path/to/directory

# Combine options
http-file-server --listen-port 9000 --dir-to-serve /path/to/directory
```

### Using Docker

#### Run the container

```bash
# Serve the current directory
docker run -p 8080:8080 -v $(pwd):/DirToServe http-file-server

# Serve a specific directory
docker run -p 8080:8080 -v /path/to/share:/DirToServe http-file-server

# Use a custom port
docker run -p 9000:8080 -v $(pwd):/DirToServe http-file-server
```

## Building from Source

```bash
# Clone the repository
git clone https://github.com/zipizap/http-file-server.git
cd http-file-server

# Build the project
./go_build.sh

# Run the server
./http-file-server
```

## Building and running the Docker Image

See `docker_build.sh` and `docker_run.sh`

## License

[MIT License](LICENSE)
