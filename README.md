# homekit-shelly

This project adds Homekit functionality to the Shelly Flood, Shelly Smoke and
Shelly Plus H&T.

You need to a MQQT service, and set both this project and your Shellies
to use it.

Remote brokers use TLS by default. Set `MQTT_HOST` to the certificate's hostname
and `MQTT_PORT` to the broker's TLS port (usually `8883`; the port default remains
`1883` for compatibility). Certificates are verified against the system trust
store. Set `MQTT_USERNAME` and `MQTT_PASSWORD` when the broker requires
credentials, and restrict publishing with broker ACLs.

The default connection to `localhost:1883`, and other numeric loopback
addresses, remains plaintext for a trusted local broker. Set `MQTT_TLS=true`
to require TLS there too. `MQTT_TLS=false` is an explicit compatibility exception
for an independently trusted transport, such as a secure tunnel. It disables
transport integrity and must not be used on an untrusted network.

Existing remote plaintext deployments must configure a TLS listener and its
port, or explicitly select that trusted-transport exception. They no longer
connect in plaintext by default.

You can then run this project with:

```bash
HT=Device1ID,Device2ID SMOKES=Device1ID,Device2ID FLOODS=Device1ID,Device2ID,etc homekit-shelly
```

Then, add the Shelly Bridge to your Home, and everything should work.

The PIN is `001-02-003`.

When an event happens in the Shellies, they will send it to MQTT, which
homekit-shelly will be listening to, and will act upon.

Previous responses are cached so it works well across restarts.
