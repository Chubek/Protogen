echo "Moving protogen executable to /usr/bin/"
echo "Moving protoquote.service && protomath.service to /etc/systemd/system/"

sudo mv ./protogen /usr/bin/
sudo mv ./protoquote.service ./protomath.service /etc/systemd/system
