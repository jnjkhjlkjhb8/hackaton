# Use MQTT over WSS with provisioned device identity

Host has no public IP but can make outbound connections, so Master Nodes connect
to Mosquitto on Host through Cloudflare Tunnel using MQTT over WSS. Each Master
is provisioned with a one-time registration token, then receives unique MQTT
credentials with topic-scoped ACLs through Mosquitto Dynamic Security. Slave
Nodes use their own transfer tokens for first use and Master reassignment. This
avoids exposing raw MQTT TCP publicly while preventing shared credentials or
unauthorised node ownership changes.
