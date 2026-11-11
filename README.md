# homekit-shelly

This project adds Homekit functionality to the Shelly Flood, Shelly Smoke and
Shelly Plus H&T.

You need to a MQQT service, and set both this project and your Shellies
to use it.

You can then run this project with:

```bash
HOMEKIT_PIN="$YOUR_PAIRING_PIN" HT=Device1ID,Device2ID SMOKES=Device1ID,Device2ID FLOODS=Device1ID,Device2ID,etc homekit-shelly
```

Then, add the Shelly Bridge to your Home, and everything should work.

Set `HOMEKIT_PIN` to a private deployment-specific value of eight decimal digits
without separators, and use it when pairing. The shared library default and
known insecure PINs are rejected. Configure this before upgrading an existing
deployment; it is required on every startup, including paired installations.

When an event happens in the Shellies, they will send it to MQTT, which
homekit-shelly will be listening to, and will act upon.

Previous responses are cached so it works well across restarts.
