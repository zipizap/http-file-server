# HTTP File Server

Very very simple and lightweight HTTP file server for uploading/downloading/deleting files in a directory.

## Overview

This HTTP File Server provides an easy way to share files and directories over HTTP. It creates a temporary web server that allows downloading/uploading/deleting files from a directory. Its meant as a basic files upload/download webpage ;)

## Usage

### Running from binary 

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

### Running from docker container

#### Building and running the Docker Image

See `docker_build.sh` and `docker_run.sh`

The container runs with uid:gid 1000:1000  (see Dockerfile)

Be mindful that the host-directory `/path/to/share` (which will be mounted as `/DirToServe` in the container) should have appropriate permissions for the container user (uid:gid) to read/write files into it.   
In last resort use `chmod a+rwX /path/to/share`.   
In any case, such conflicts would produce errors in the logs, with hints about the symptoms.   


```bash
# Serve a specific directory
docker run -p 8080:8080 -v /path/to/share:/DirToServe http-file-server

# Serve the current directory
docker run -p 8080:8080 -v $(pwd):/DirToServe http-file-server

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

## Running behind nginx reverse proxy

To run the HTTP file server behind an nginx reverse proxy, you can use the following configuration as an example.

The out-of-ordinary parameters are the `proxy_read_timeout` and `client_max_body_size` which are important for file uploads to work correctly.

```nginx
server {
    listen 443 ssl;
    server_name myfiles.example.com;

    ssl_certificate /etc/nginx/certs/myfiles.example.com-cert.pem;
    ssl_certificate_key /etc/nginx/certs/myfiles.example.com-key.pem;

    location / {
        proxy_pass http://webserved:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        # These 2 options bellow are important for file uploads to work correctly
        proxy_read_timeout 310s;
        client_max_body_size 0;
    }
}
```

## License

[Affero AGPL](LICENSE)
