Step #1: Download the built executable for x86_64 AnyLinux from the release section.

Step #2: Exctract the tar.xz or zip file in your server

Step #3: Run `sudo chmod +x ./install.sh` and then `./install.sh` or combine these two steps into one and run `sudo bash ./install.sh`

Step #4: Now `protogen` is moved to /usr/bin and `protoquote.service` is moved to your `etc/systemd/system` folder.

Step #5: Now run `sudo systemcl daemon-reload && sudo systemctl enable protoquote && sudo systemctl start protoquote` --- this will enable and run the protoquote service. 

Now protoquote is running on TCP port 8888. To restart and stop use `sudo systemctl restart protoquote` and `sudo systemctl stop protoquote` respectively

If you wish so, both `install.sh` and `protoquote.service` are inside the `sciprts` folder.