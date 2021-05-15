systemctl stop gocacher
cp gocacher /usr/local/bin/
cp systemd/gocacher.service /etc/systemd/system/gocacher.service
systemctl daemon-reload
systemctl start gocacher
