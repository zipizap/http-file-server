set -xefu
docker run -v ./testFolder:/DirToServe -p 8080:8080 http-file-server:latest