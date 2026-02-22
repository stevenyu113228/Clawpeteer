# Broker Setup Guide

This guide walks through setting up the Mosquitto MQTT broker for Clawpeteer.

## 1. Install Mosquitto

**macOS:**
```bash
brew install mosquitto
```

**Debian/Ubuntu:**
```bash
sudo apt-get update
sudo apt-get install mosquitto mosquitto-clients
```

**RHEL/CentOS:**
```bash
sudo yum install mosquitto
```

## 2. Run the Setup Script

The fastest way to configure the broker is with the provided setup script:

```bash
cd broker
sudo ./setup.sh
```

This script will:
- Verify Mosquitto is installed
- Create required directories (`/etc/mosquitto/certs`, `/var/lib/mosquitto`, `/var/log/mosquitto`)
- Copy `mosquitto.conf` and `acl` to `/etc/mosquitto/`
- Create the `openclaw` user (the controller) and a default `agent` user
- Generate self-signed TLS certificates for development
- Set appropriate file permissions

You will be prompted to set passwords for each user.

### Manual Setup

If you prefer to configure manually:

```bash
# Copy config files
sudo cp broker/mosquitto.conf /etc/mosquitto/mosquitto.conf
sudo cp broker/acl /etc/mosquitto/acl

# Create the password file
sudo mosquitto_passwd -c /etc/mosquitto/passwd openclaw
sudo mosquitto_passwd /etc/mosquitto/passwd agent
```

## 3. Add Agent Users

Each remote machine needs its own MQTT user. The username should match the agent's `agentId` in its config:

```bash
sudo mosquitto_passwd /etc/mosquitto/passwd home-pc
sudo mosquitto_passwd /etc/mosquitto/passwd office-server
```

The ACL file uses pattern-based rules, so each agent automatically gets access only to its own topics (matching `%u` to the username).

## 4. Test the Connection

Start the broker:
```bash
# Foreground (for testing)
mosquitto -c /etc/mosquitto/mosquitto.conf

# Or with systemd
sudo systemctl start mosquitto
sudo systemctl enable mosquitto
```

Test the non-TLS local connection:
```bash
# Terminal 1: Subscribe
mosquitto_sub -h 127.0.0.1 -p 1883 -u openclaw -P <password> -t 'clawpeteer/#'

# Terminal 2: Publish
mosquitto_pub -h 127.0.0.1 -p 1883 -u openclaw -P <password> -t 'clawpeteer/test' -m 'hello'
```

Test the TLS connection:
```bash
mosquitto_sub -h localhost -p 8883 \
  --cafile /etc/mosquitto/certs/ca.crt \
  -u openclaw -P <password> \
  -t 'clawpeteer/#'
```

## 5. Production Considerations

### Firewall

Only expose port 8883 (TLS) externally. Keep port 1883 bound to localhost:

```bash
# UFW example
sudo ufw allow 8883/tcp
sudo ufw deny 1883/tcp
```

### Cloudflare Tunnel

For remote access without opening ports, use a Cloudflare Tunnel:

```bash
# Install cloudflared
# Create a tunnel pointing to localhost:8883
cloudflared tunnel create clawpeteer-mqtt
cloudflared tunnel route dns clawpeteer-mqtt mqtt.yourdomain.com
cloudflared tunnel run clawpeteer-mqtt
```

Then configure agents to connect to `mqtts://mqtt.yourdomain.com:8883`.

### Certificate Management

For production, replace the self-signed certificates with ones from a real CA (e.g., Let's Encrypt):

```bash
# Update mosquitto.conf to point to your real certs
certfile /etc/letsencrypt/live/mqtt.yourdomain.com/fullchain.pem
keyfile /etc/letsencrypt/live/mqtt.yourdomain.com/privkey.pem
```

### Monitoring

Check the broker log for connection issues:
```bash
tail -f /var/log/mosquitto/mosquitto.log
```

Monitor connected clients:
```bash
mosquitto_sub -h 127.0.0.1 -p 1883 -u openclaw -P <password> -t '$SYS/broker/clients/connected'
```
