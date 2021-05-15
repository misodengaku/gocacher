systemctl stop gocacher
docker build -t gocacher .
cp systemd/gocacher-docker.service /etc/systemd/system/gocacher.service
systemctl daemon-reload
systemctl start gocacher
